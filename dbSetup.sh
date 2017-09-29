#!/bin/bash

user="root"
pass="password"
db="fileHashes"
table="hashes"

mysql -u$user -p$pass -e "create database if not exists $db"
mysql -u$user -p$pass -e "SET sql_mode = '';"
mysql -u$user -p$pass -D$db -e "create table if not exists hashes(filePath varchar(500),
      extension varchar(10), permissions varchar(12), hash varchar(50),
      hashTime timestamp default '0000-00-00 00:00:00',
      lastModified timestamp default '0000-00-00 00:00:00',
      oldHash varchar(50),
      oldTime timestamp default '0000-00-00 00:00:00',
      primary key(filePath) );"
