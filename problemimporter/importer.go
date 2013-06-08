package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"labix.org/v2/mgo"
	"strconv"
	"github.com/contester/runlib/mongotools"
	"io/ioutil"
	"log"
	"sort"
)

type ProblemManifest struct {
	Id string
	Revision int
	TestCount int `bson:"testCount"`
	TimeLimitMicros int64 `bson:"timeLimitMicros"`
	MemoryLimit int64 `bson:"memoryLimit"`
	Stdio bool `bson:"stdio"`
	TesterName string `bson:"testerName"`
	Answers []int `bson:"answers"`
	InteractorName string `bson:"interactorName,omitempty"`
}

func storeIfExists(mfs *mgo.GridFS, filename, gridname string) error {
	if _, err := os.Stat(filename); err != nil {
		return err
	}

	err := mongotools.GridfsCopy(filename, gridname, mfs, true)
	if err != nil {
		return err
	}
	return nil
}

func importProblem(id, root, gridprefix string, mdb *mgo.Database, mfs *mgo.GridFS) error {
	var manifest ProblemManifest

	manifest.Id = "moodle/" + id
	manifest.Revision = 1

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

		if err = storeIfExists(mfs, filepath.Join(testRoot, "Input", "input.txt"), gridprefix + "tests/" + strconv.FormatInt(testId, 10) + "/input.txt"); err != nil {
			continue
		}

		if err = storeIfExists(mfs, filepath.Join(testRoot, "Add-ons", "answer.txt"), gridprefix + "tests/" + strconv.FormatInt(testId, 10) + "/answer.txt"); err == nil {
			manifest.Answers = append(manifest.Answers, int(testId))
		}

		if int(testId) < manifest.TestCount {
			manifest.TestCount = int(testId)
		}
	}

	if err = storeIfExists(mfs, filepath.Join(root, "Tester", "tester.exe"), gridprefix + "checker"); err != nil {
		return err
	}

	manifest.TesterName = "tester.exe"

	memlimitString, err := ioutil.ReadFile(filepath.Join(root, "memlimit"))
	if err == nil {
		fmt.Println(string(memlimitString))
		manifest.MemoryLimit, err = strconv.ParseInt(string(memlimitString), 10, 64)
	} else {
		fmt.Println(err)
	}

	timexString, err := ioutil.ReadFile(filepath.Join(root, "timex"))
	if err == nil {
		fmt.Println(string(timexString))
		timex, err := strconv.ParseFloat(string(timexString), 64)
		if err == nil {
			manifest.TimeLimitMicros = int64(timex * 1000000)
		}
	} else {
		fmt.Println(err)
	}

	if manifest.Answers != nil {
		sort.Ints(manifest.Answers)
	}

	fmt.Println(manifest)

	return mdb.C("manifest").Insert(&manifest)
}

func importProblems(root string, mdb *mgo.Database, mfs *mgo.GridFS) error {
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

		err = importProblem(strconv.FormatUint(problemId, 10), problem, "problem/moodle/" + strconv.FormatUint(problemId, 10) + "/1/", mdb, mfs)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	mhost := flag.String("mongohost", "", "")

	flag.Parse()

	if *mhost == "" {
		return
	}

	msession, err := mgo.Dial(*mhost)
	if err != nil {
		log.Fatal(err)
	}
	mdb := msession.DB("contester")
	mfs := mdb.GridFS("fs")

	err = importProblems(flag.Arg(0), mdb, mfs)
	if err != nil {
		log.Fatal(err)
	}
}
