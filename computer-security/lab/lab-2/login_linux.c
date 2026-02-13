/* $Header:
 * https://svn.ita.chalmers.se/repos/security/edu/course/computer_security/trunk/lab/login_linux/login_linux.c
 * 585 2013-01-19 10:31:04Z pk@CHALMERS.SE $ */

/* gcc -std=gnu99 -Wall -g -o mylogin login_linux.c -lcrypt */

#include <crypt.h>
#include <pwd.h>
#include <signal.h>
#include <stdio.h>
#include <stdio_ext.h>
#include <stdlib.h>
#include <string.h>
#include <sys/types.h>
#include <unistd.h>
/* Uncomment next line in step 2 */
#include "pwent.h"

#define TRUE 1
#define FALSE 0
#define LENGTH 16

// helper (msg) function
void terminate_program(int sig) {
  printf("\nQuitting program...\n");
  exit(0); // Clean exit with status 0
}

void sighandler() {
  /* add signalhandling routines here */
  /* see 'man 2 signal' */
  // sigquit
  signal(SIGQUIT, terminate_program);
  // ignore Ctrl + C and Ctrl + Z
  signal(SIGINT, SIG_IGN);
  signal(SIGTSTP, SIG_IGN);
}

int main(int argc, char *argv[]) {
  // using struct from file pwent.h
  databaseEntry *userEntry;
  char important1[LENGTH] = "**IMPORTANT 1**";
  char user[LENGTH];
  char important2[LENGTH] = "**IMPORTANT 2**";

  // char   *c_pass; //you might want to use this variable later...
  char prompt[] = "password: ";
  char *userPassword;

  sighandler();

  while (TRUE) {
    /* check what important variable contains - do not remove, part of buffer
     * overflow test */
    // printf("Value of variable 'important1' before input of login name: %s\n",
    // 	   important1);
    // printf("Value of variable 'important2' before input of login name: %s\n",
    // 	   important2);
    printf("login: ");
    fflush(NULL);    /* Flush all  output buffers */
    __fpurge(stdin); /* Purge any data in stdin buffer */
    // the gets function decrapped, suggested to change to fgets
    // if (gets(user) == NULL) /* gets() is vulnerable to buffer */
    // 	exit(0); /*  overflow attacks.  */
    // using fgets to avoid buffer overflow
    if (fgets(user, sizeof(user), stdin) == NULL) {
      exit(0); // temp exit, have not been implemented yet
    }

    // workaround since the input contains \n (newline)
    // and string must be ended with \0 (null terminated)
    user[strcspn(user, "\n")] = 0;

    // save the result into a struct
    userEntry = getDatabaseEntry(user);

    if (userEntry == NULL) {
      printf("[ERROR] There is no user matched with this name\n");
      continue;
    }

    /* check to see if important variable is intact after input of login name -
     * do not remove */
    // printf("Value of variable 'important 1' after input of login name:
    // %*.*s\n", 	   LENGTH - 1, LENGTH - 1, important1); printf("Value of
    // variable 'important 2' after input of login name: %*.*s\n",
    // LENGTH - 1, LENGTH - 1, important2);

    userPassword = getpass(prompt);

    // this method is only to get from a pre-define database, not suitable for
    // step 2 here this program will use a simple db (struct) for encrypted
    // stored passwd

    // passwddata = getpwnam(user);

    // if (passwddata != NULL) {
    // 	/* You have to encrypt user_pass for this to work */
    // 	/* Don't forget to include the salt */

    // 	if (!strcmp(user_pass, passwddata->pw_passwd)) {

    // 		printf(" You're in !\n");

    // 		/*  check UID, see setuid(2) */
    // 		/*  start a shell, use execve(2) */

    // 	}

    char *encryptedPassword = crypt(userPassword, userEntry->passwordSalt);

    if (strcmp(userEntry->password, encryptedPassword) == 0) {
      printf("[SUCCESS] You're in !\n");
      printf("[CHECK] The user's UID is: %d\n", userEntry->uid);

      // also reset failed counter (regardless age)
      userEntry->attemptsFailed = 0;
      if (updateDatabaseEntry(user, userEntry) == -1) {
        printf("[ERROR] Could not update the entry.\n");
      }

      // update entry here
      userEntry->passwordAge++;

      if (getDatabaseEntry(user)->passwordAge >= 10) {
        printf("[INFO] You should change password now...\n");
        char *new_pass;
        char new_prompt[] = "new password: ";
        new_pass = getpass(new_prompt);

        userEntry->passwordAge = 0;
        userEntry->password = new_pass;

        if (updateDatabaseEntry(user, userEntry) == -1) {
          printf("[ERROR] Could not update the entry.\n");
        }
      }

      int res = setuid(userEntry->uid);
      if (res == -1) {
        perror("Setuid failed");
        exit(1);
      }

      char *argv[] = {"/bin/sh", NULL};
      char *envp[] = {NULL};

      execve("/bin/sh", argv, envp);

      // error catch
      perror("execve failed");
      exit(1);
    }

    // if we go here, increase the incorrect login, maybe
    // prompt it?
    // update value first
    userEntry->attemptsFailed = userEntry->attemptsFailed + 1;
    if (updateDatabaseEntry(user, userEntry) == -1) {
      printf("[ERROR] Could not update the entry.\n");
    }

    printf(
        "[ERROR] Login Incorrect, you have %d times failed login attempts. \n",
        userEntry->attemptsFailed);
  }
  return 0;
}
