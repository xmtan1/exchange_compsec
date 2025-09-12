#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define MAX_LEN 10

int main(int argc, char* argv[]){
	char input[MAX_LEN + 1];
	printf("The init string: %s\n", input);
	printf("The source to be copied: %s\n", argv[1]);
	size_t newlen = strlen(argv[1]);
	input[newlen]='\0';
	printf("The len of holder: %ld\n", strlen(input));
	for(int i = 0; argv[1][i] != '\0'; i++){
		printf("Copy the character %c, with position %d\n", argv[1][i], i);
		input[i] = argv[1][i];
	}
	input[sizeof(argv[1]) + 1] = '\0';
	printf("The destination: %s\n", input);
	printf("Size comparison, source %ld, dest %ld\n", strlen(argv[1]), strlen(input));
	return 0;
}
