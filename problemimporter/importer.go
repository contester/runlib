package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/contester/runlib/storage"
	"log"
	"sort"
	"strconv"
	"strings"
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

	_, err := backend.Copy(filename, gridname, true, "", "")
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

	tests, err := filepath.Glob(filepath.Join(root, "Test.*"))
	if err != nil {
		return err
	}

	for _, testRoot := range tests {
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
		fmt.Println(memlimitString)
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
		fmt.Println(timexString)
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
	problems, err := filepath.Glob(filepath.Join(root, "Task.*"))
	if err != nil {
		return err
	}
	for _, problem := range problems {
		ext := filepath.Ext(problem)

		if len(ext) < 2 {
			continue
		}

		problemId, err := strconv.ParseUint(ext[1:], 10, 32)
		if err != nil {
			continue
		}

		realProblemId := "direct://school.sgu.ru/moodle/" + strconv.FormatUint(problemId, 10)

		err = importProblem(realProblemId, problem, backend)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	storageUrl := flag.String("url", "", "")
	mode := flag.String("mode", "", "")

	flag.Parse()

	if *storageUrl == "" {
		return
	}

	stor := storage.NewStorage()
	err := stor.SetDefault(*storageUrl)
	if err != nil {
		log.Fatal(err)
	}

	backend := stor.Default.(storage.ProblemStore)

	if *mode == "import" {
		err = importProblems(flag.Arg(0), backend)
		if err != nil {
			log.Fatal(err)
		}
	}

	if *mode == "cleanup" {
		backend.Cleanup(1)
	}
}
