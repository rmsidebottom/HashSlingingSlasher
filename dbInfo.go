package main

var database = "fileHashes"
var table = "hashes"
var username = "root"
var password = "password"
var protocol = "tcp(127.0.0.1:3306)"
var dbStatement string = username + ":" + password + "@" + protocol + "/" + database
