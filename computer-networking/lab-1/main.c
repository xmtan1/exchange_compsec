#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <assert.h>
#include <netdb.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <netdb.h>
#include <arpa/inet.h>


#define MAX_LEN 1024

int main(int argc, char *argv[]){
    // Declare hint attributes with flag, socktype and protocol
    struct addrinfo hints = {0};
    hints.ai_flags = AF_INET;
    hints.ai_socktype = SOCK_STREAM;
    hints.ai_protocol = IPPROTO_TCP;
    
    // Declare array of result and a pointer in case of resolving more than one sub-hostname
    struct addrinfo *result;
    struct addrinfo *result_pointer;
    
    char* input_host = argv[1];

    // Message as required
    // Debug message
    // printf("The input string is: %s\n",argv[1]);
    if(MAX_LEN == 0){
        printf("Max length for hostname if not set, error!\n");
        exit(EXIT_FAILURE);
    }

    if(argc < 2){
        printf("Need a hostname to resolve, error!\n");
        exit(EXIT_FAILURE);
    }

    char hostname[MAX_LEN + 1];
    gethostname(hostname, MAX_LEN);

    printf("Resolving '%s' from '%s':\n",input_host,hostname);
    // printf("Test for printing addrinfo struct property: %d %d %d \n", hints.ai_flags, hints.ai_socktype, hints.ai_protocol);

    int error = getaddrinfo(argv[1], NULL, &hints, &result);

    // printf("Testing the result of getaddinfo command: %d\n", error);
    // error = 1;
    if (error != 0){
        printf("The program got an error from getaddrinfo() function: %s\n", gai_strerror(error));
        exit(EXIT_FAILURE);
    }
    // Iterate through the result array
    for (result_pointer = result; result_pointer != NULL; result_pointer = result_pointer->ai_next){
        char ipaddress[MAX_LEN + 1];
        struct sockaddr* const res_sock = result_pointer->ai_addr;
        
        // assert(AF_INET == res_sock->sa_family);

        if (res_sock->sa_family == AF_INET){
            struct sockaddr_in* res_addr = (struct sockaddr_in*)res_sock;
            printf("IPv4: %s\n", inet_ntop(AF_INET, &res_addr->sin_addr, ipaddress, sizeof(ipaddress)));
        }
        else if (res_sock->sa_family == AF_INET6){
            struct sockaddr_in6* res_addr = (struct sockaddr_in6*)res_sock;
            printf("IPv6: %s\n", inet_ntop(AF_INET6, &res_addr->sin6_addr, ipaddress, sizeof(ipaddress)));
        }  
        
        // struct sockaddr_in* res_addr = (struct sockaddr_in*)res_sock;
        // printf("IPv4: %s\n", inet_ntop(AF_INET, &res_addr->sin_addr, ipaddress, sizeof(ipaddress)));
    }

    freeaddrinfo(result);

    return 0;
}