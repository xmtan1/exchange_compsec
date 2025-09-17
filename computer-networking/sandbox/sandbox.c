#include <stdio.h>
#include <unistd.h>
#include <getopt.h>


int main (int argc, char *argv[]) {
  int character;
  char *options = "h";
  int longindex;
  int moartest_flag = 0;

  struct option longopts[] = {
    {"help", no_argument, NULL, 'h'},
    {"echo", required_argument, NULL, 0},
    {"longtest", optional_argument, &moartest_flag, 12},
    {NULL, 0, NULL, 0}
  };

  while((character = getopt_long(argc, argv, options, longopts, &longindex)) != -1) {
    printf("getopt_long returned: '%c' (%d)\n", character, character);
    switch (character) {
      case 'h':
        printf("help!\n");
        break;
      case 0:
        printf("longindex: %d\n", longindex);
        printf("longopts[longindex].name = %s\n", longopts[longindex].name);
        printf("optarg: %s\n", optarg);
        printf("moartest_flag: %d\n", moartest_flag);
    }
  }

  return 0;
}