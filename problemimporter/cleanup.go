package main

import (
	"fmt"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"net/url"
	"strings"
)

func getAllProblemIds(mdb *mgo.Database) []string {
	var ids []string
	mdb.C("manifest").Find(nil).Distinct("id", &ids)
	return ids
}

func idToGridPrefix(id string) string {
	u, err := url.Parse(id)
	if err != nil {
		return ""
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		return "problem/polygon/" + u.Scheme + "/" + u.Host + "/" + u.Path
	}
	if u.Scheme == "direct" {
		return "problem/direct/" + u.Host + "/" + u.Path
	}
	return ""
}

func doCleanup(id string, latest int, mdb *mgo.Database) error {
	iter := mdb.C("manifest").Find(bson.M{"id": id}).Sort("-revision").Iter()
	defer iter.Close()
	var manifest ProblemManifest

	for iter.Next(&manifest) {
		if latest--; latest >= 0 {
			continue
		}
		mdb.C("manifest").RemoveId(manifest.MongoId)
	}
	return nil
}

func getAllGridPrefixes(mdb *mgo.Database) []string {
	var ids []string
	iter := mdb.C("manifest").Find(nil).Iter()
	defer iter.Close()
	var m ProblemManifest
	for iter.Next(&m) {
		ids = append(ids, m.GetGridPrefix())
	}
	return ids
}

func doAllCleanup(latest int, mdb *mgo.Database, mfs *mgo.GridFS) error {
	pids := getAllProblemIds(mdb)
	for _, v := range pids {
		doCleanup(v, latest, mdb)
	}

	pids = getAllGridPrefixes(mdb)
	fmt.Println(pids)
	iter := mfs.Find(nil).Sort("filename").Iter()
	var f *mgo.GridFile
	for mfs.OpenNext(iter, &f) {
		fmt.Println(f.Name())
		if !strings.HasPrefix(f.Name(), "problem/") {
			continue
		}
		for _, v := range pids {
			if strings.HasPrefix(f.Name(), v) {
				fmt.Printf("Remove: %s\n", f.Name())
				mfs.RemoveId(f.Id())
			}
		}
	}
	return nil
}
