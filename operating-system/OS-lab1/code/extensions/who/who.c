#include <stdio.h>
#include <stdlib.h>

int main()
{
    printf("test\n");

    const char* s = getenv("USER");
    const char* s1 = getenv("LOGNAME");

    // If the environment variable doesn't exist, it returns NULL
    printf("USER :%s\t LOGNAME :%s\n", (s != NULL) ? s : "getenv returned NULL", (s1 != NULL) ? s1 : "getenv returned NULL");

    printf("end test\n");
}