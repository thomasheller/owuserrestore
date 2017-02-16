# Oxwall User Restore (owuserrestore)

[![Go Report Card](https://goreportcard.com/badge/github.com/thomasheller/owuserrestore)](https://goreportcard.com/report/github.com/thomasheller/owuserrestore)

If you accidentially deleted a user from your Oxwall (www.oxwall.com)
install (or the user deleted his/her account and now wants it back), 
this script helps you recovering most user data. In fact, it merges
your current database dump with an any older database dump where the
user data still exists. This script is only a "best guess" and in no
way supported by the Oxwall team, so proceed with caution!

## Install

Prerequisites:
  - Git
  - Golang 
  
If you haven't already, install Git and Golang on your system. On
Ubuntu/Debian this would be:

```
sudo apt-get install git golang
```

Then set up Go:
  - Create a directory for your `$GOPATH`, for example `~/gocode`
  - Set the `$GOPATH` environment variable accordingly: `export GOPATH=~/gocode`
  - Add the `bin` directory to your `$PATH`, for example: ` export PATH=$PATH:~/gocode/bin`
  
Now you can install owuserrestore using `go get`:

```
go get github.com/thomasheller/owuserrestore
```
 
## Database setup

owuserrestore requires three distinct MySQL databases:
  1. `owuserrestoreold` Old database containing the user data you'd like to restore
  2. `owuserrestorecurrent` Current database missing the user's data
  3. `owuserrestoremerge` Another copy of the current database

Use a local MySQL server to run owuserrestore. Never run it against a live system.
Instead, put your Oxwall site in maintenance mode, pull a database dump from its
server, and re-enable your site after you've verified that everything works fine.

If you don't have a local MySQL server running already, the following command will
install `mysql-server` on Ubuntu/Debian:

```
sudo apt-get install mysql-server
```

You can import your database dumps into three distinct databases as follows:

Run MySQL shell using `mysql -uUSERNAME -pPASSWORD` (where `USERNAME` and `PASSWORD`
are whatever you chose during the MySQL install) and issue the following commands,
replacing `YOUROLDDUMPFILE.sql` and `YOURCURRENTDUMPFILE.sql` with the filenames of
your database dumps:

```
create database owuserrestoreold;
use owuserrestoreold;
source YOUROLDDUMPFILE.sql;
create database owuserrestorecurrent;
use owuserrestorecurrent;
source YOURCURRENTDUMPFILE.sql;
create database owuserrestoremerge;
use owuserrestoremerge;
source YOURCURRENTDUMPFILE.sql;
quit;
```

If anything goes wrong or you'd like to start over, remember dropping the database(s)
using `drop database DB;` (where `DB` is `owuserrestoreold`, `owuserrestorenew`
`owuserrestoremerge`) in MySQL shell first.

owuserrestore will copy the user's data from the first database (`owuserrestoreold`)
which do not exist in the second database (`owuserrestorecurrent`) into the third
database (`owuserrestoremerge`)

**Important:** All three databases must follow the same database schema -- however,
owuserrestore will not verify that. You can check that manually, for instance in
MySQL workbench (`sudo apt-get install mysql-workbench`) via
`Database > Compare Schemas` (only available when viewing a model/ER diagram).

## Merging the databases

If all three databases are in place, run owuserrestore as follows, replacing `12345`
with the of the user you'd like to restore (also, replace `USERNAME` and `PASSWORD`
with whatever you chose during MySQL install):

```
owuserrestore -userid 12345 -dbolduser USERNAME -dboldpass PASSWORD -dbcurrentuser USERNAME -dbcurrentpass PASSWORD -dbmergeuser USERNAME -dbmergepass PASSWORD 
```

owuserrestore will print some statistics regarding the differences between the
databases. To actually do the merge, use the `-real` flag:

```
owuserrestore -real -userid 12345 -dbolduser USERNAME -dboldpass PASSWORD -dbcurrentuser USERNAME -dbcurrentpass PASSWORD -dbmergeuser USERNAME -dbmergepass PASSWORD 
```

## Epilogue

Once owuserrestore is done, get a dump of the merged database, so you can replace
your live database:

```
mysqldump -uUSERNAME -pPASSWORD owuserrestoremerge > owuserrestoremerge.sql
```
