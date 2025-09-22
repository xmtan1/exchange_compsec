#include <assert.h>
#include <stdio.h>
#include <time.h>
#include <stdio.h>
#include <stdlib.h>
#include <getopt.h>

// int main(void) {
//     time_t t = time(NULL);
//     struct tm *tm = localtime(&t);
//     char s[64];
//     size_t ret = strftime(s, sizeof(s), "%c", tm);
//     assert(ret);
//     printf("%s\n", s);
//     return 0;
// }

int main(int argc, char* argv[]){
    int ret;
    //posilbe options
    char* options = "lh";

    struct option longopts[] = {
        {"help", no_argument, NULL, 'h'},
        {"long", no_argument, NULL, 'l'},
        {NULL, 0, NULL, 0}
    };

    time_t t = time(NULL);
    struct tm *tm = localtime(&t);
    char s[64];

    while((ret = getopt_long(argc, argv, options, longopts, NULL)) != -1){
        switch (ret){
        case 'h':
            printf("This is the help message for this program %s\n", argv[0]);
            printf("Use -l or --long to display the date and time in long format.\n");
            printf("Use -h or --help to display this help message.\n");
            exit(EXIT_SUCCESS);
        case 'l':
            size_t ret = strftime(s, sizeof(s), "%c", tm);
            assert(ret);
            printf("Current date time: %s\n", s);
            exit(EXIT_SUCCESS);
        case '?':
            fprintf(stderr, "Unknown option: %c\n", optopt);
            printf("Use -h or --help to see the help message.\n");
            exit(EXIT_FAILURE);
        default:
            fprintf(stderr, "Error parsing options\n");
            exit(EXIT_FAILURE);
        }
    }

    printf("Current local time : %d:%d:%d\n", tm->tm_hour, tm->tm_min, tm->tm_sec);

    return 0;
}