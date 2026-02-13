/* $Header:
 * https://svn.ita.chalmers.se/repos/security/edu/course/computer_security/trunk/lab/login_linux/makepass.c
 * 584 2013-01-19 10:30:22Z pk@CHALMERS.SE $ */

/* register.c - Register a user in the local database */
/* compile: gcc -std=gnu99 -Wall -g -o register register.c -lcrypt */
/* usage: "register" */

#include "pwent.h"
#include <crypt.h>
#include <pwd.h>
#include <signal.h>
#include <stdio.h>
#include <stdio_ext.h>
#include <stdlib.h>
#include <string.h>
#include <sys/types.h>
#include <unistd.h>

#define TRUE 1
#define FALSE 0
#define LENGTH 16

int main(int argc, char *argv[]) {
  // using struct from file pwent.h
  databaseEntry *userEntry;
  char name[LENGTH];
  char uid[sizeof(int) * 8]; // How big an int can be character wise
  char *password;
  char *confirmPassword;

  while (TRUE) {
    printf("----- REGISTER -----\n");
    fflush(NULL);    /* Flush all  output buffers */
    __fpurge(stdin); /* Purge any data in stdin buffer */

    printf("Choose a name: ");
    if (fgets(name, sizeof(name), stdin) == NULL) {
      exit(0); // temp exit, have not been implemented yet
    }

    // workaround since the input contains \n (newline)
    // and string must be ended with \0 (null terminated)
    name[strcspn(name, "\n")] = 0;

    // Check if entry already exists
    if ((userEntry = getDatabaseEntry(name)) != NULL) {
      printf("[ERROR] There is a user already with this name\n");
      continue;
    }

    printf("Choose a uid: ");
    if (fgets(uid, sizeof(uid), stdin) == NULL) {
      exit(0); // temp exit, have not been implemented yet
    }
    int uidInt = atoi((char *)uid);

    // Get password
    password = getpass("Password: ");
    confirmPassword = getpass("Retype password: ");

    if (strcmp(password, confirmPassword) != 0) {
      printf("[ERROR] Passwords aren't matching!\n");
      continue;
    }

    if (appendDatabaseEntry(name, uidInt, password) != 0) {
      printf("[ERROR] Couldn't register user...\n");
      continue;
    }

    printf("[SUCCESS] User registered correctly!\n");
  }
  return 0;
}
