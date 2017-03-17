# Schemaman

Schemaman is an experiment in trying to find a better way to track database
schemas in git and generate SQL migration patches for people who don't use ORMs.

It models the database schema as a collection of git tree objects, and uses
git diff-tree (or cat-file) to automatically generate SQL patches between
arbitrary commits (or dump the entire schema at any revision.)

## Example

```sh
# Create a table in a database
$ mysql
CREATE TABLE foo (
	ID integer
);
exit;

# Import it into the .schema directory and commit it to git.
$ schemaman import [..connection options]
$ git commit -a

# Create another table, and modify the existing one, then commit it.
$ mysql 
CREATE TABLE bar (
	ID integer,
	Val varchar(255)
	
);
ALTER TABLE foo ADD COLUMN atime datetime;
exit;
$ schemaman import [..connection options]
$ git commit

# Print a dump of the most recent table schema in git HEAD.
$ schemaman show 
CREATE TABLE foo (
	ID int(11),
	atime datetime
) ENGINE = InnoDB;

CREATE TABLE bar (
	ID int(11),
	Val varchar(255)
) ENGINE = InnoDB;

# Or print a dump of the schema at any arbitrary commit.
$ schemaman show HEAD^

CREATE TABLE foo (
	ID int(11)
) ENGINE = InnoDB;

# Generate an SQL patch between two arbitrary commits.
$ schemaman diff HEAD^ HEAD

CREATE TABLE bar (
	ID int(11),
	Val varchar(255)
) ENGINE = InnoDB;
ALTER TABLE foo ADD COLUMN atime datetime;

# The above wasn't very exciting since it's exactly what we typed in, but we can
# also generate a patch in the opposite direction (or in a real repo with more
# than two commits between any two commits that we wanted.)
$ schemaman diff HEAD HEAD^
DROP TABLE bar;
ALTER TABLE foo DROP COLUMN atime;
```

