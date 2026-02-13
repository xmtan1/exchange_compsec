/* $Header:
 * https://svn.ita.chalmers.se/repos/security/edu/course/computer_security/trunk/lab/login_linux/pwent.h
 * 586 2013-01-19 10:32:53Z pk@CHALMERS.SE $ */

/* pwent.h - Password entry header file */

/* These routines write and read records from a password database,
 named "passdb" in the current directory.

 The 'mygetpwnam' routine takes as argument a username, and returns
 NULL on failure to find it in the database, or a pointer to static
 storage if found. This storage will be overwritten on the next
 call.

 The 'mysetpwent' routine takes a name, and a struct 'mypwent', and
 replaces the data pertaining to 'name' in the database with the
 supplied struct. It returns 0 on success, -1 on failure to replace
 the record.

 The database has free form records, the length of which must not
 exceed 79 characters, the fields are separated by ':', much like
 the passwd database. The fields are:
 name:uid:passwd:salt:no_of_failed_attempts:password_age respectively.

 Note the separate 'salt' field, to simplify some of non-obligatory
 assignments, it is of course entirely possible, not to use this
 field, but instead to include the salt in the password field, in
 similarity with the passwd database.  */

/* Usage: copy the files to your own working directory, and
 use; #include "pwent.h"   to include it. */

#ifndef PWENT_H
#define PWENT_H

/* Definition for the database */
#define DATABASE_FILENAME "passdb"
#define DATABASE_TMP_FILENAME "passdb.tmp"
// A string column in the database should only be 1000 ascii characters long
#define DATABASE_STRING_LENGTH (sizeof(char) * 1000)
#define DATABASE_PASSWORD_SALT_LENGTH (sizeof(char) * 100)

// The entry should be three strings, and three integers as per the definition
// below.
#define DATABASE_ENTRY_LENGTH (DATABASE_STRING_LENGTH * 3 + sizeof(int) * 3)

// Format of an entry is:
// "username:userID:password:passwordSalt:attemptsFailed:passwordAge"
#define DATABASE_ENTRY_SCAN_FORMAT "%[^:]:%d:%[^:]:%[^:]:%d:%d"
#define DATABASE_ENTRY_SET_FORMAT "%s:%d:%s:%s:%d:%d\n"

typedef struct {
  char *name;         /* Username */
  int uid;            /* User id */
  char *password;     /* Password */
  char *passwordSalt; /* Make dictionary attack harder */
  int attemptsFailed; /* No. of failed attempts */
  int passwordAge;    /* Age of password in no of logins */
} databaseEntry;

databaseEntry *getDatabaseEntry(char *name); /* Find entry matching username */
int updateDatabaseEntry(
    char *name,
    databaseEntry *newDatabaseEntry); /* Set entry based on username */
int appendDatabaseEntry(
    char *name, int uid,
    char *password); // Append entry to the end of the database

#endif
