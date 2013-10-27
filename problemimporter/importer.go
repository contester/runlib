package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/contester/runlib/mongotools"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"log"
	"sort"
	"strconv"
	"strings"
)

type ProblemManifest struct {
	MongoId         string `bson:"_id"`
	Id              string
	Revision        int
	TestCount       int    `bson:"testCount"`
	TimeLimitMicros int64  `bson:"timeLimitMicros"`
	MemoryLimit     int64  `bson:"memoryLimit"`
	Stdio           bool   `bson:"stdio"`
	TesterName      string `bson:"testerName"`
	Answers         []int  `bson:"answers"`
	InteractorName  string `bson:"interactorName,omitempty"`
	CombinedHash    string `bson:"combinedHash,omitempty"`
}

func (s *ProblemManifest) GetGridPrefix() string {
	return idToGridPrefix(s.Id) + "/" + strconv.FormatInt(int64(s.Revision), 10) + "/"
}

func storeIfExists(mfs *mgo.GridFS, filename, gridname string) error {
	if _, err := os.Stat(filename); err != nil {
		return err
	}

	_, err := mongotools.GridfsCopy(filename, gridname, mfs, true, "", "")
	if err != nil {
		return err
	}
	return nil
}

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

func getNextRevision(id string, mdb *mgo.Database) (int, error) {
	query := mdb.C("manifest").Find(bson.M{"id": id}).Sort("-revision")
	var manifest ProblemManifest
	if err := query.One(&manifest); err != nil {
		return 1, nil
	}
	return manifest.Revision + 1, nil
}

func importProblem(id, root string, mdb *mgo.Database, mfs *mgo.GridFS) error {
	var manifest ProblemManifest
	var err error

	manifest.Id = id
	manifest.Revision, err = getNextRevision(id, mdb)
	manifest.MongoId = manifest.Id + "/" + strconv.FormatInt(int64(manifest.Revision), 10)

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

		if err = storeIfExists(mfs, filepath.Join(testRoot, "Input", "input.txt"), gridprefix+"tests/"+strconv.FormatInt(testId, 10)+"/input.txt"); err != nil {
			continue
		}

		if err = storeIfExists(mfs, filepath.Join(testRoot, "Add-ons", "answer.txt"), gridprefix+"tests/"+strconv.FormatInt(testId, 10)+"/answer.txt"); err == nil {
			manifest.Answers = append(manifest.Answers, int(testId))
		}

		if int(testId) > manifest.TestCount {
			manifest.TestCount = int(testId)
		}
	}

	if err = storeIfExists(mfs, filepath.Join(root, "Tester", "tester.exe"), gridprefix+"checker"); err != nil {
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

		realProblemId := "direct://moodle/school.sgu.ru/" + strconv.FormatUint(problemId, 10)

		err = importProblem(realProblemId, problem, mdb, mfs)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	mhost := flag.String("mongohost", "", "")
	mode := flag.String("mode", "", "")

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

	if *mode == "import" {

		err = importProblems(flag.Arg(0), mdb, mfs)
		if err != nil {
			log.Fatal(err)
		}
	}

	if *mode == "cleanup" {
		doAllCleanup(1, mdb, mfs)
	}
}
