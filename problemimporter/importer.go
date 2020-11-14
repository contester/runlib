package main

import (
	"archive/zip"
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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

func storeIfExists(backend storage.Backend, filename, gridname string) error {
	if _, err := os.Stat(filename); err != nil {
		return err
	}

	_, err := backend.Copy(filename, gridname, true, "", "", *authToken)
	if err != nil {
		return err
	}
	return nil
}

func importProblem(id, root string, backend storage.ProblemStore) error {
	var manifest storage.ProblemManifest
	var err error

	manifest.Id = id
	manifest.Revision, err = backend.GetNextRevision(id)
	manifest.Key = manifest.Id + "/" + strconv.FormatInt(int64(manifest.Revision), 10)

	gridprefix := manifest.GetGridPrefix()

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

		if err = storeIfExists(backend, filepath.Join(testRoot, "Input", "input.txt"),
			gridprefix+"tests/"+strconv.FormatInt(testId, 10)+"/input.txt"); err != nil {
			continue
		}

		if err = storeIfExists(backend, filepath.Join(testRoot, "Add-ons", "answer.txt"),
			gridprefix+"tests/"+strconv.FormatInt(testId, 10)+"/answer.txt"); err == nil {
			manifest.Answers = append(manifest.Answers, int(testId))
		}

		if int(testId) > manifest.TestCount {
			manifest.TestCount = int(testId)
		}
	}

	if err = storeIfExists(backend, filepath.Join(root, "Tester", "tester.exe"), gridprefix+"checker"); err != nil {
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

	return backend.SetManifest(&manifest)
}

func importProblems(root string, backend storage.ProblemStore) error {
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

		err = importProblem(realProblemId, filepath.Join(root, problemShort), backend)
		if err != nil {
			return err
		}
	}
	return nil
}

func writeRemoteAs(w *zip.Writer, backend storage.ProblemStore, name, as string) error {
	fi, err := backend.ReadRemote(name, *authToken)
	if err != nil {
		return err
	}
	defer fi.Body.Close()
	fh := zip.FileHeader{
		Name:               as,
		UncompressedSize64: uint64(fi.Stat.Size_),
		Method:             zip.Deflate,
	}
	wr, err := w.CreateHeader(&fh)
	if err != nil {
		return err
	}
	_, err = io.Copy(wr, fi.Body)
	return err
}

func writeBytesAs(w *zip.Writer, as string, data []byte) error {
	fh := zip.FileHeader{
		Name:               as,
		UncompressedSize64: uint64(len(data)),
		Method:             zip.Deflate,
	}
	wr, err := w.CreateHeader(&fh)
	if err != nil {
		return err
	}
	_, err = wr.Write(data)
	return err
}

func exportProblem(w *zip.Writer, backend storage.ProblemStore, manifest storage.ProblemManifest, dest string) error {
	gridprefix := manifest.GetGridPrefix()
	if manifest.TesterName != "" {
		if err := writeRemoteAs(w, backend, gridprefix+"checker", filepath.Join(dest, "Tester", manifest.TesterName)); err != nil {
			return err
		}
	}
	if err := writeBytesAs(w, filepath.Join(dest, "memlimit"), []byte(fmt.Sprintf("%d", manifest.MemoryLimit))); err != nil {
		return err
	}
	if err := writeBytesAs(w, filepath.Join(dest, "timex"), []byte(fmt.Sprintf("%f", float64(manifest.TimeLimitMicros)/1000000))); err != nil {
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
		test := filepath.Join(dest, fmt.Sprintf("Test.%d", i))
		if err := writeRemoteAs(w, backend, gridprefix+fmt.Sprintf("tests/%d/input.txt", i), filepath.Join(test, "Input", "input.txt")); err != nil {
			return err
		}
		if _, ok := answers[i]; !ok {
			continue
		}
		if err := writeRemoteAs(w, backend, gridprefix+fmt.Sprintf("tests/%d/answer.txt", i), filepath.Join(test, "add-ons", "answer.txt")); err != nil {
			return err
		}
	}
	return nil
}

func exportProblems(backend storage.ProblemStore, destfile string) error {
	m, err := backend.GetAllManifests()
	if err != nil {
		return err
	}

	probs := make(map[int64]storage.ProblemManifest)

	for _, v := range m {
		if !strings.HasPrefix(v.Id, "direct://school.sgu.ru/moodle/") {
			continue
		}
		pidstr := strings.TrimPrefix(v.Id, "direct://school.sgu.ru/moodle/")
		pidint, err := strconv.ParseInt(pidstr, 10, 64)
		if err != nil {
			continue
		}
		if prev, ok := probs[pidint]; !ok || prev.Revision < v.Revision {
			probs[pidint] = v
		}
	}

	outf, err := os.Create(destfile)
	if err != nil {
		return err
	}
	defer outf.Close()

	zw := zip.NewWriter(outf)
	defer zw.Close()

	for pidint, v := range probs {
		fmt.Printf("Exporting problem %d ... [", pidint)
		os.Stdout.Sync()
		if err = exportProblem(zw, backend, v, fmt.Sprintf("task.%d", pidint)); err != nil {
			return err
		}
		fmt.Printf("]\n")
	}

	return nil
}

func fixMemoryLimit(backend storage.ProblemStore, newLimit int64) error {
	manifests, err := backend.GetAllManifests()
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
		if err = backend.SetManifest(&m); err != nil {
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
		err = importProblems(flag.Arg(0), backend)
	case "cleanup":
		backend.Cleanup(1)
	case "export":
		err = exportProblems(backend, flag.Arg(0))
	case "fixMemoryLimit":
		var newMl int64
		if newMl, err = strconv.ParseInt(flag.Arg(0), 10, 64); newMl > 0 {
			err = fixMemoryLimit(backend, newMl)
		}
	}
	if err != nil {
		log.Fatal(err)
	}
}
