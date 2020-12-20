package main

import (
	"archive/zip"
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/contester/runlib/storage"
)

func readFirstLine(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	r := bufio.NewScanner(f)

	if r.Scan() {
		return strings.TrimSpace(r.Text()), nil
	}
	return "", nil
}

func storeIfExists(ctx context.Context, backend storage.Backend, filename, gridname string) error {
	if _, err := os.Stat(filename); err != nil {
		return err
	}

	_, err := backend.Copy(ctx, filename, gridname, true, "", "", *authToken)
	if err != nil {
		return err
	}
	return nil
}

func importProblem(ctx context.Context, id, root string, backend storage.ProblemStore, urlPrefix string) error {
	var manifest storage.ProblemManifest
	var err error

	manifest.Id = id
	manifest.Revision, err = backend.GetNextRevision(ctx, id)
	manifest.Key = manifest.Id + "/" + strconv.FormatInt(int64(manifest.Revision), 10)

	gridprefix := urlPrefix + manifest.GetGridPrefix()

	rootDir, err := os.Open(root)
	if err != nil {
		return err
	}

	names, err := rootDir.Readdirnames(-1)

	for _, shortName := range names {
		if !strings.HasPrefix(strings.ToLower(shortName), "test.") {
			continue
		}
		testRoot := filepath.Join(root, shortName)
		if dstat, err := os.Stat(testRoot); err != nil || !dstat.IsDir() {
			continue
		}

		ext := filepath.Ext(testRoot)
		if len(ext) < 2 {
			continue
		}

		testId, err := strconv.ParseInt(ext[1:], 10, 32)
		if err != nil {
			continue
		}

		if err = storeIfExists(ctx, backend, filepath.Join(testRoot, "Input", "input.txt"),
			gridprefix+"tests/"+strconv.FormatInt(testId, 10)+"/input.txt"); err != nil {
			continue
		}

		if err = storeIfExists(ctx, backend, filepath.Join(testRoot, "Add-ons", "answer.txt"),
			gridprefix+"tests/"+strconv.FormatInt(testId, 10)+"/answer.txt"); err == nil {
			manifest.Answers = append(manifest.Answers, int(testId))
		}

		if int(testId) > manifest.TestCount {
			manifest.TestCount = int(testId)
		}
	}

	if err = storeIfExists(ctx, backend, filepath.Join(root, "Tester", "tester.exe"), gridprefix+"checker"); err != nil {
		return err
	}

	manifest.TesterName = "tester.exe"

	memlimitString, err := readFirstLine(filepath.Join(root, "memlimit"))
	if err == nil {
		manifest.MemoryLimit, err = strconv.ParseInt(string(memlimitString), 10, 64)
		if err != nil {
			fmt.Println(err)
		}
		if manifest.MemoryLimit < 16*1024*1024 {
			manifest.MemoryLimit = 16 * 1024 * 1024
		}
	} else {
		fmt.Println(err)
	}

	timexString, err := readFirstLine(filepath.Join(root, "timex"))
	if err == nil {
		timex, err := strconv.ParseFloat(string(timexString), 64)
		if err == nil {
			manifest.TimeLimitMicros = int64(timex * 1000000)
		} else {
			fmt.Println(err)
		}
	} else {
		fmt.Println(err)
	}

	if manifest.Answers != nil {
		sort.Ints(manifest.Answers)
	}

	fmt.Println(manifest)

	return backend.SetManifest(ctx, &manifest)
}

func importProblems(ctx context.Context, root string, backend storage.ProblemStore, urlPrefix string) error {
	rootDir, err := os.Open(root)
	if err != nil {
		return err
	}
	problems, err := rootDir.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, problemShort := range problems {
		if !strings.HasPrefix(strings.ToLower(problemShort), "task.") {
			continue
		}
		ext := filepath.Ext(problemShort)

		if len(ext) < 2 {
			continue
		}

		problemId, err := strconv.ParseUint(ext[1:], 10, 32)
		if err != nil {
			continue
		}

		realProblemId := "direct://school.sgu.ru/moodle/" + strconv.FormatUint(problemId, 10)

		err = importProblem(ctx, realProblemId, filepath.Join(root, problemShort), backend, urlPrefix)
		if err != nil {
			return err
		}
	}
	return nil
}

type localWriter interface {
	Close() error
	OpenProblem(id int) (localProblemWriter, error)
}

type localProblemWriter interface {
	Close() error
	WriteChecker(name string, rf *storage.RemoteFile) error
	WriteLimits(m storage.ProblemManifest) error
	WriteInput(testID int, rf *storage.RemoteFile) error
	WriteAnswer(testID int, rf *storage.RemoteFile) error
}

type problemAssetType int

type localZipWriter struct {
	w *zip.Writer
}

func (s *localZipWriter) OpenProblem(id int) (localProblemWriter, error) {
	if id <= 0 {
		return nil, fmt.Errorf("id must be > 0")
	}
	return &localZipPw{
		w:      s.w,
		prefix: "Task." + strconv.Itoa(id),
	}, nil
}

type localZipPw struct {
	w      *zip.Writer
	prefix string
}

func (s *localZipPw) Close() error {
	return nil
}

func (s *localZipPw) WriteInput(testID int, rf *storage.RemoteFile) error {
	return s.writeLocal(filepath.Join("Test."+strconv.Itoa(testID), "input.txt"), rf)
}

func (s *localZipPw) WriteAnswer(testID int, rf *storage.RemoteFile) error {
	return s.writeLocal(filepath.Join("Test."+strconv.Itoa(testID), "answer.txt"), rf)
}

func (s *localZipPw) WriteChecker(name string, rf *storage.RemoteFile) error {
	return s.writeLocal(filepath.Join("Tester", name), rf)
}

func (s *localZipPw) WriteLimits(m storage.ProblemManifest) error {
	if err := s.writeBytes("memlimit", []byte(strconv.FormatInt(m.MemoryLimit, 10))); err != nil {
		return err
	}
	return s.writeBytes("timex", []byte(strconv.FormatFloat(float64(m.TimeLimitMicros)/1000000, 'f', -1, 64)))
}

func (s *localZipPw) writeBytes(as string, b []byte) error {
	as = filepath.Join(s.prefix, as)
	fh := zip.FileHeader{
		Name:               as,
		UncompressedSize64: uint64(len(b)),
		Method:             zip.Deflate,
	}
	wr, err := s.w.CreateHeader(&fh)
	if err != nil {
		return err
	}
	_, err = wr.Write(b)
	return err
}
func (s *localZipPw) writeFile(as string, size uint64, body io.Reader) error {
	as = filepath.Join(s.prefix, as)
	fh := zip.FileHeader{
		Name:               as,
		UncompressedSize64: size,
		Method:             zip.Deflate,
	}
	wr, err := s.w.CreateHeader(&fh)
	if err != nil {
		return err
	}
	_, err = io.Copy(wr, body)
	return err
}

func (s *localZipPw) writeLocal(as string, rf *storage.RemoteFile) error {
	return s.writeFile(as, uint64(rf.Stat.GetSize_()), rf.Body)
}

func withRemoteFile(ctx context.Context, remote string, f func(rf *storage.RemoteFile) error) error {
	nctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	fi, err := storage.FilerReadRemote(nctx, remote, *authToken)
	if err != nil {
		return err
	}
	defer fi.Body.Close()
	return f(fi)
}

func exportProblem(ctx context.Context, w localProblemWriter, manifest storage.ProblemManifest, baseURL string) error {
	gridprefix := manifest.GetGridPrefix()
	probprefix := baseURL + gridprefix
	if manifest.TesterName != "" {
		if err := withRemoteFile(ctx, probprefix+"checker", func(rf *storage.RemoteFile) error {
			return w.WriteChecker(manifest.TesterName, rf)
		}); err != nil {
			return err
		}
	}

	if err := w.WriteLimits(manifest); err != nil {
		return err
	}

	answers := make(map[int]struct{})
	for _, v := range manifest.Answers {
		answers[v] = struct{}{}
	}

	for i := 1; i <= manifest.TestCount; i++ {
		if i > 1 {
			fmt.Printf(" ")
		}
		fmt.Printf("%d", i)
		os.Stdout.Sync()

		testprefix := probprefix + "tests/" + strconv.Itoa(i) + "/"

		if err := withRemoteFile(ctx, testprefix+"input.txt", func(rf *storage.RemoteFile) error {
			return w.WriteInput(i, rf)
		}); err != nil {
			return err
		}

		if _, ok := answers[i]; !ok {
			continue
		}
		if err := withRemoteFile(ctx, testprefix+"answer.txt", func(rf *storage.RemoteFile) error {
			return w.WriteAnswer(i, rf)
		}); err != nil {
			return err
		}
	}
	return nil
}

func exportProblems(ctx context.Context, backend storage.ProblemStore, baseURL, destfile string) error {
	m, err := backend.GetAllManifests(ctx)
	if err != nil {
		return err
	}

	probs := make(map[int]storage.ProblemManifest)

	for _, v := range m {
		if !strings.HasPrefix(v.Id, "direct://school.sgu.ru/moodle/") {
			continue
		}
		pidstr := strings.TrimPrefix(v.Id, "direct://school.sgu.ru/moodle/")
		pidint, err := strconv.ParseInt(pidstr, 10, 64)
		if err != nil {
			continue
		}
		if prev, ok := probs[int(pidint)]; !ok || prev.Revision < v.Revision {
			probs[int(pidint)] = v
		}
	}

	outf, err := os.Create(destfile)
	if err != nil {
		return err
	}
	defer outf.Close()

	zw := zip.NewWriter(outf)
	defer zw.Close()
	lzw := &localZipWriter{w: zw}

	for pidint, v := range probs {
		fmt.Printf("Exporting problem %d ... [", pidint)
		os.Stdout.Sync()

		pw, err := lzw.OpenProblem(pidint)
		if err != nil {
			return err
		}

		if err = exportProblem(ctx, pw, v, baseURL); err != nil {
			return err
		}
		pw.Close()
		fmt.Printf("]\n")
	}

	return nil
}

func fixMemoryLimit(ctx context.Context, backend storage.ProblemStore, newLimit int64) error {
	manifests, err := backend.GetAllManifests(ctx)
	if err != nil {
		return err
	}
	for _, m := range manifests {
		if !strings.HasPrefix(m.Id, "direct://school.sgu.ru/moodle/") {
			continue
		}
		if m.MemoryLimit >= newLimit {
			continue
		}
		m.MemoryLimit = newLimit
		fmt.Printf("%+v\n", &m)
		if *dryRun {
			continue
		}
		if err = backend.SetManifest(ctx, &m); err != nil {
			return err
		}
	}
	return nil
}

var (
	storageUrl = flag.String("url", "", "")
	mode       = flag.String("mode", "", "")
	authToken  = flag.String("auth_token", "", "")
	dryRun     = flag.Bool("dry_run", false, "")
)

func main() {

	flag.Parse()

	if *storageUrl == "" {
		return
	}

	stor, err := storage.NewBackend(*storageUrl)
	if err != nil {
		log.Fatal(err)
	}

	backend := stor.(storage.ProblemStore)

	switch *mode {
	case "import":
		err = importProblems(context.Background(), flag.Arg(0), backend, *storageUrl+"fs/")
	case "export":
		err = exportProblems(context.Background(), backend, *storageUrl+"fs/", flag.Arg(0))
	case "fixMemoryLimit":
		var newMl int64
		if newMl, err = strconv.ParseInt(flag.Arg(0), 10, 64); newMl > 0 {
			err = fixMemoryLimit(context.Background(), backend, newMl)
		}
	}
	if err != nil {
		log.Fatal(err)
	}
}
