/* $Header:
 * https://svn.ita.chalmers.se/repos/security/edu/course/computer_security/trunk/lab/login_linux/pwent.c
 * 584 2013-01-19 10:30:22Z pk@CHALMERS.SE $ */

/*
 A simple library for password databases.  Tobias Gedell 2007
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include "pwent.h"

/*
 Return pointer to password entry for specified user.

 Upon error, or if the user couldn't be found, NULL is returned.

 Note: The returned pointer points to static data.
 */
databaseEntry *getDatabaseEntry(char *name) {
  FILE *file;
  char entryBuffer[DATABASE_STRING_LENGTH];

  // printf("Comparison name: %s", name);

  static char nameBuffer[DATABASE_STRING_LENGTH],
      passwordBuffer[DATABASE_STRING_LENGTH],
      passwordSaltBuffer[DATABASE_STRING_LENGTH];
  static databaseEntry entry = {nameBuffer,         0, passwordBuffer,
                                passwordSaltBuffer, 0, 0};

  /* Open file, return NULL if it failed. */
  if ((file = fopen(DATABASE_FILENAME, "rb")) == NULL) {
    return NULL;
  }

  /* Read each line, looking for the right entry. */
  while (fgets(entryBuffer, sizeof(entryBuffer), file) != NULL) {
    if (sscanf(entryBuffer, DATABASE_ENTRY_SCAN_FORMAT, entry.name, &entry.uid,
               entry.password, entry.passwordSalt, &entry.attemptsFailed,
               &entry.passwordAge) != 6)
      break;

    if (strcmp(nameBuffer, name) == 0) {
      fclose(file);
      return &entry;
    }
  }

  fclose(file);

  return NULL;
}

/*
 Update password entry for user.

 Upon error, or if the user couldn't be found, -1 is returned,
 otherwise 0.
 */
int updateDatabaseEntry(char *name, databaseEntry *newDatabaseEntry) {
  FILE *file;
  FILE *tmpFile;
  char entryBuffer[DATABASE_ENTRY_LENGTH];
  char nameBuffer[DATABASE_STRING_LENGTH];

  int status = -1;

  if ((file = fopen(DATABASE_FILENAME, "rb")) == NULL)
    return -1;
  if ((tmpFile = fopen(DATABASE_TMP_FILENAME, "wb")) == NULL) {
    fclose(file);
    return -1;
  }

  /* Read each line, looking for the right entry. */
  while (fgets(entryBuffer, sizeof(entryBuffer), file) != NULL) {
    if (sscanf(entryBuffer, "%[^:]", nameBuffer) != 1) {
      status = -1;
      break;
    }

    /* See if we found the entry to be updated. */
    if (strcmp(nameBuffer, name) == 0) {
      if (snprintf(entryBuffer, sizeof(entryBuffer), DATABASE_ENTRY_SET_FORMAT,
                   newDatabaseEntry->name, newDatabaseEntry->uid,
                   newDatabaseEntry->password, newDatabaseEntry->passwordSalt,
                   newDatabaseEntry->attemptsFailed,
                   newDatabaseEntry->passwordAge) >= sizeof(entryBuffer)) {
        status = -1;
        break;
      }

      status = 0;
    }

    if (fputs(entryBuffer, tmpFile) < 0) {
      status = -1;
      break;
    }
  }

  fclose(tmpFile);
  fclose(file);

  /* Swap files if user successfully updated. */
  if (status == 0) {
    if (rename(DATABASE_TMP_FILENAME, DATABASE_FILENAME) != 0)
      status = -1;
  } else
    unlink(DATABASE_TMP_FILENAME);

  return status;
}

/*
 Register an entry for a new user.

 Upon error, or if the user couldn't be found, -1 is returned,
 otherwise 0.
 */
int appendDatabaseEntry(char *name, int uid, char *password) {
  FILE *file;
  char entryBuffer[DATABASE_ENTRY_LENGTH];

  if ((file = fopen(DATABASE_FILENAME, "ab")) == NULL)
    return -1;

  // Create pseudo-random password salt
  char *passwordSalt = (char *)malloc(DATABASE_PASSWORD_SALT_LENGTH);

  for (int i = 0; i < DATABASE_PASSWORD_SALT_LENGTH / sizeof(char); i++) {
    // Create a salt where each character is between 'A-Z' or 'a-z' randomly
    if (random() % 2)
      passwordSalt[i] = 'A' + random() % 26;
    else
      passwordSalt[i] = 'a' + random() % 26;
  }

  databaseEntry newDatabaseEntry = {name, uid, password, passwordSalt, 0, 0};

  // Append the entry to the end of the database.
  if (snprintf(entryBuffer, sizeof(entryBuffer), DATABASE_ENTRY_SET_FORMAT,
               newDatabaseEntry.name, newDatabaseEntry.uid,
               newDatabaseEntry.password, newDatabaseEntry.passwordSalt,
               newDatabaseEntry.attemptsFailed,
               newDatabaseEntry.passwordAge) >= sizeof(entryBuffer)) {
    free(passwordSalt);
    return -1;
  }

  if (fputs(entryBuffer, file) < 0) {
    free(passwordSalt);
    return -1;
  }

  free(passwordSalt);
  fclose(file);

  unlink(DATABASE_TMP_FILENAME);

  return 0;
}
