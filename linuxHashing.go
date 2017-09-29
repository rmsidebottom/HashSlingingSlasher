/*
*   file: hashing.go
*   Description: Program will hash every file on the file system. With the
*   modification of a couple lines, can be used on both Windows and *nix.
*   Requirements: Needs to be compiled with dbInfo.go as dbInfo.go has the
*   pertinent database information needed to connect to the database.
*   Recommendation: Create a new table in the database for each system being
*   checked.
*   Author: Ryan Sidebottom
*   Date: 07/05/2017
 */

package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// the file struct, contains all the hash information for the file
type fileHashInfo struct {
	path         string
	extension    string
	permissions  string
	hash         string
	hashTime     string
	modifiedDate string
	oldHash      string
	oldTime      string
}

//create connection to the DB, do it here so only one connection needed
var con, err = sql.Open("mysql", dbStatement)

// channel to store fileinfo to be sent to DB
const fileChanSize = 250

var files = make(chan fileHashInfo, fileChanSize)

// channel to store errors which will be written to
var errors = make(chan string)
var errorLog string = "error.log"

//constants
const numOfDrives = 3  // sets the max number of goroutines in relation to drives
const numOfFiles = 100 // set max num of goroutines in relation to files

/// Program starts here
/// Connects to database, initiates search on file system
/// the main go thread
func main() {
	runtime.GOMAXPROCS(runtime.NumCPU() / 2)
	//Defer will close the connection when main ends
	defer con.Close()
	if err != nil {
		fmt.Printf("Database error: %v \n", err)
		return
	}

	var logging bool = true
	errLog, err := os.Create(errorLog)
	if err != nil {
		fmt.Printf("Cannot open Error Log: %v \n", err)
		logging = false
	}
	defer errLog.Close()

	go func() {
		defer close(errors)
		defer fmt.Println("ERRORS CLOSED")
		if logging {
			for e := range errors {
				fmt.Fprintf(errLog, e)
			}
		}
	}()

	// create channel to be used that will contain the hash information for all 100
	// files at a time
	go func() {
		defer close(files)
		done := make(chan bool)
		filePath := "/Users/daboss/go/"
		start := fmt.Sprintf("%s", filePath)
		// fmt.Println(start)
		go func() {
			err := filepath.Walk(start, fileFunc)
			if err != nil {
				fmt.Println(err)
			}
			done <- true
		}()
		<-done
	}()

	count := 0
	insert := fmt.Sprintf("insert into %s (filePath, extension, "+
		"permissions, hash, hashTime, lastModified) values ",
		table)
	// read from channel, send to database
	for x := range files {
		if count < fileChanSize {
			response := toDB(x)
			if response != "" {
				if count == 0 {
					count++
					insert += fmt.Sprintf("%s", response)
				} else {
					count++
					insert += fmt.Sprintf(", %s", response)
				}
			}
		} else {
			count = 0
			// fmt.Println(insert)
			con.Exec(insert)
			insert = fmt.Sprintf("insert into %s (filePath, extension, "+
				"permissions, hash, hashTime, lastModified) values ",
				table)
		}
	}
	con.Exec(insert)
	// fmt.Println(insert)
}

/// this function is called for every file the program finds in each one of the
/// drives
/// from here a go routine is made for each file to simply make a hash and add
/// the end result to the open channel
func fileFunc(path string, _ os.FileInfo, _ error) error {
	var errMsg string
	fileDir, err := os.Stat(path)
	if err != nil {
		errMsg = "FILE ERROR :::::: " + err.Error() + " \n"
		errors <- errMsg
		return nil
	}

	if !fileDir.IsDir() {
		func() error {
			/// Will hash the requested file as well as update/modify the database
			/// path is the aboslute location of the file
			/// no return because it passes the data to the database
			// attempt to open file
			file, err := os.Open(path)
			if err != nil {
				errMsg = "OPEN ERROR :::::: " + err.Error() + " \n"
				errors <- errMsg
				return nil
			}

			// defer passes this call to when function ends
			defer file.Close()

			// result will be stored here
			var hashResult string
			// generate new md5 hash interface
			hash := md5.New()

			// attempt to copy file in hash interface
			_, err = io.Copy(hash, file)
			// if error exists can't continue
			if err != nil {
				errMsg = "FILE ERROR :::::: " + err.Error() + " \n"
				errors <- errMsg
				return nil
			}

			// setting up the variables for submission
			hashResult = hex.EncodeToString(hash.Sum(nil)[:16])
			var hashTime = time.Now().UTC().Format("2006-01-02 15:04:05.000000")
			var dateFileModified = fileDir.ModTime().UTC().Format("2006-01-02 15:04:05")
			var ext = filepath.Ext(path)
			var perms = fileDir.Mode().String()

			var fHashInfo = fileHashInfo{path, ext, perms, hashResult, hashTime, dateFileModified, "--", "--"}
			files <- fHashInfo
			return nil
		}()
	}

	return nil
}

/// Function prepares a statement to be sent over to the database
func toDB(file fileHashInfo) string {
	sqlStmt := "select filePath, hash, hashTime, lastModified from " + table +
		" where filePath = \"" + file.path + "\""
	row := con.QueryRow(sqlStmt)

	var dbFilePath, dbFileHash, dbHashTime, dbDateModified string
	err = row.Scan(&dbFilePath, &dbFileHash, &dbHashTime, &dbDateModified)
	fmt.Println(err)

	// if rows == nil, no entry, add new one
	if err == sql.ErrNoRows {
		//IF ITS NEW, ADD TO A QUEUE TO BULK INSERT
		sqlStmt = fmt.Sprintf("('%s', '%s', '%s', '%s', '%s', '%s')",
			file.path, file.extension, file.permissions, file.hash,
			file.hashTime, file.modifiedDate)
		//return the sql code
		fmt.Println(sqlStmt)
		return sqlStmt
	} else {
		//OTHERWISE SIMPLY UPDATE THE ENTRY
		sqlStmt = "update " + table + " set oldHash = '" + dbFileHash +
			"', oldTime = '" + dbHashTime + "', hash = '" +
			file.hash + "', hashTime = '" + file.hashTime +
			"', lastModified = '" + file.modifiedDate + "', " +
			"extension = \"" + file.extension + "\", permissions = \"" +
			file.permissions + "\" where filePath = \"" + file.path + "\""
		_, err = con.Exec(sqlStmt)
		fmt.Println(sqlStmt)
		if err != nil {
			errorMsg := fmt.Sprintf("DB Update error:::: %s \n", err.Error())
			errors <- errorMsg
		}
		return ""
	}
}
