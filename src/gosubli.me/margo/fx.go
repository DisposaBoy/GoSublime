package main

import (
	"path/filepath"
	"strings"
)

var (
	fileExts = map[string]void{}
)

func init() {
	exts := []string{
		".c",
		".h",
		".go",
		".goc",
		".md",
		".txt",
		".git",
		".bzr",
		".tmp",
		".swig",
		".swigcxx",
		".a",
		".s",
		".S",
		".syso",
		".so",
		".dll",
		".o",
		".5",
		".6",
		".8",
		".out",
		".cc",
		".hh",
		".dat",
		".py",
		".pyc",
		".zip",
		".z",
		".7z",
		".gz",
		".tar",
		".bz2",
		".tgz",
		".rar",
		".pro",
		".occ",
		".asc",
		".conf",
		".html",
		".jpg",
		".png",
		".js",
		".json",
		".src",
		".log",
		".patch",
		".diff",
		".php",
		".rit",
		".css",
		".lua",
		".less",
		".ttf",
		".expected",
		".ps",
		".bak",
		".cix",
		".d",
		".hac",
		".hrb",
		".java",
		".lexres",
		".lst",
		".pan",
		".phpt",
		".prof",
		".set",
		".sol",
		".vrs",
		".exe",
		".bat",
		".sh",
		".rc",
		".bash",
	}

	for _, ext := range exts {
		fileExts[ext] = void{}
	}
}

func fx(nm string) (isFileExt, isGoFileExt bool) {
	nm = strings.ToLower(nm)
	ext := filepath.Ext(nm)

	if ext == ".go" {
		return true, true
	}

	if !strings.HasPrefix(nm, "go.") {
		if _, ok := fileExts[ext]; ok {
			return true, false
		}
	}

	return false, false
}
