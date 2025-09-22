/*
 * Main source code file for lsh shell program
 *
 * You are free to add functions to this file.
 * If you want to add functions in a separate file(s)
 * you will need to modify the CMakeLists.txt to compile
 * your additional file(s).
 *
 * Add appropriate comments in your code to make it
 * easier for us while grading your assignment.
 *
 * Using assert statements in your code is a great way to catch errors early and make debugging easier.
 * Think of them as mini self-checks that ensure your program behaves as expected.
 * By setting up these guardrails, you're creating a more robust and maintainable solution.
 * So go ahead, sprinkle some asserts in your code; they're your friends in disguise!
 *
 * All the best!
 */
#include <assert.h>
#include <ctype.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <readline/readline.h>
#include <readline/history.h>

// The <unistd.h> header is your gateway to the OS's process management facilities.
#include <unistd.h>

// Allow the used of several system-related calls
#include <sys/types.h>
#include <sys/wait.h>

#include "parse.h"

#define MAX_LEN 256

static void print_cmd(Command *cmd);
static void print_pgm(Pgm *p);
void stripwhite(char *);
int resolve_cmd(const char* command_path, char* processed_path, size_t max_len);


// a pointer array to store custom libs
char* custom_libs[MAX_LEN] = {
    "/home/huyhoang-ph/msc-cybersecurity/operating-system/OS-lab1/code/lib_x64",
    NULL
};

int main(void){ 
  // declare a command to process later, depended on background or not
  Command to_process;
  // prompt current working directory for debug with native cd command
  // if directory changes happened later, will modified in the below code
  char cwd[MAX_LEN + 1];

  // recognize the PATH environment variable
  char* path_env = getenv("PATH");
  if(path_env != NULL){
    printf("PATH environment variable: %s\n", path_env);
  }
  else{
    printf("PATH environment variable not found.\n");
  }
  
  for (;;){
    if(getcwd(cwd, sizeof(cwd))== NULL){
      perror("getcwd() error");
      return -1;
    }

    // for (int i = 0; custom_libs[i] != NULL; i++){
    //   printf("Custom lib path %d: %s\n", i, custom_libs[i]);
    // }

    // printf("%s \n", cwd);
    char *line;
    line = readline("> ");

    // handle EOF - Ctrl + D signal
    if(line == NULL){
      printf("End of file - EOF detected, quit the program.\n");
      free(line);
      return 0;
    }

    // Remove leading and trailing whitespace from the line
    stripwhite(line);

    // If stripped line not blank
    if (*line)
    {
      add_history(line);

      Command cmd;
      if (parse(line, &cmd) == 1)
      {
        // Just prints cmd
        // print_cmd(&cmd);
        to_process = cmd;
      }
      else
      {
        printf("Parse ERROR\n");
      }
    }

    // begin to process the command
    Pgm *p = to_process.pgm;
    while (p != NULL){
      // printf("Current command to execute: %s\n", *(p->pgmlist));

      // handle exit first
      if (strcmp(*(p->pgmlist), "exit") == 0){
        printf("Exit command detected, terminating the shell.\n");
        free(line);
        return 0;
      }

      // handle built-in command of clear
      if (strcmp(*(p->pgmlist), "clear") == 0){
        // system("clear"); // use system call to clear the terminal
        // Or use ANSI escape codes to clear the terminal
        printf("\033[H\033[J");
        p = p->next;
        continue;
      }

      // for debugging purpose - chdir command
      if(strcmp(*(p->pgmlist), "pwd") == 0){
        printf("%s \n", cwd);
        p = p->next;
        continue;
      }

    if (strcmp(*(p->pgmlist), "cd") == 0) {
        // if no argument, go to home directory
        if (p->pgmlist[1] == NULL) {
          char *home_dir = getenv("HOME");
          if (home_dir != NULL) {
            if (chdir(home_dir) != 0) {
                perror("chdir to HOME failed");
            }
          } 
          else {
            fprintf(stderr, "HOME environment variable not set.\n");
          }
        } 
        else {
          // go to the passed directory
          if (chdir(p->pgmlist[1]) != 0)
            perror("chdir failed");
        }

        p = p->next;
        continue;
      }

      pid_t pid = fork();
      
      if (pid < 0){
        perror("Fork failed");
        free(line);
        return -1;
      }

      if(pid == 0){
        char command_path[MAX_LEN];
        if(resolve_cmd(p->pgmlist[0], command_path, sizeof(command_path)) != 0){
          fprintf(stderr, "Command not found: %s\n", p->pgmlist[0]);
          exit(EXIT_FAILURE);
        }

        // printf("Executing command: %s\n", command_path);

        execv(command_path, p->pgmlist);
        perror("exec failed");
        exit(EXIT_FAILURE);
      }
      else{
        // printf("Parent process, child PID = %d\n", pid);
        waitpid(pid, NULL, 0);
      }

      p = p->next;
    }
    

    // Clear memory
    free(line);
  }
  return 0;
}

/*
 * Print a Command structure as returned by parse on stdout.
 *
 * Helper function, no need to change. Might be useful to study as inspiration.
 */
static void print_cmd(Command *cmd_list)
{
  printf("------------------------------\n");
  printf("Parse OK\n");
  printf("stdin:      %s\n", cmd_list->rstdin ? cmd_list->rstdin : "<none>");
  printf("stdout:     %s\n", cmd_list->rstdout ? cmd_list->rstdout : "<none>");
  printf("background: %s\n", cmd_list->background ? "true" : "false");
  printf("Pgms:\n");
  print_pgm(cmd_list->pgm);
  printf("------------------------------\n");
}

/* Print a (linked) list of Pgm:s.
 *
 * Helper function, no need to change. Might be useful to study as inpsiration.
 */
static void print_pgm(Pgm *p)
{
  if (p == NULL)
  {
    return;
  }
  else
  {
    char **pl = p->pgmlist;

    /* The list is in reversed order so print
     * it reversed to get right
     */
    print_pgm(p->next);
    printf("            * [ ");
    while (*pl)
    {
      printf("%s ", *pl++);
    }
    printf("]\n");
  }
}


/* Strip whitespace from the start and end of a string.
 *
 * Helper function, no need to change.
 */
void stripwhite(char *string)
{
  size_t i = 0;

  while (isspace(string[i]))
  {
    i++;
  }

  if (i)
  {
    memmove(string, string + i, strlen(string + i) + 1);
  }

  i = strlen(string) - 1;
  while (i > 0 && isspace(string[i]))
  {
    i--;
  }

  string[++i] = '\0';
}

/**
 * Hanlde passing path of a filename, with a backup is pointer to the custom libs path
 * @param lib_paths: a pointer array of library paths
 * @return the first valid library path, or NULL if none found
 */
int resolve_cmd(const char* command_path, char* processed_path, size_t max_len){
  // check if the command path is appropriate (with slash /)
  if (strchr(command_path, '/')){
    if(access(command_path, X_OK) == 0){
      strcpy(processed_path, command_path);
      return 0;
    }
  }

  // if the command path is not appropriate, check in the custom libs
  for (int i = 0; custom_libs[i] != NULL; i++){
    // pass the command path to the custom lib
    snprintf(processed_path, max_len, "%s/%s", custom_libs[i], command_path);
    if(access(processed_path, X_OK) == 0){
      return 0;
    }
  }

  return -1;
}

