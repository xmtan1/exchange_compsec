# official image from GOLANG
FROM golang:latest

# Create a new user just for running the application
RUN groupadd -g 1123 application && \
    useradd -u 1123 -g application application

# Print the UID and GID
RUN echo 'Inside Container:' && echo 'User: $(whoami) UID: $(id -u) GID: $(id -g)'

WORKDIR /usr/src/app
COPY . .
EXPOSE 8080
RUN cd advance-server && go build -v

RUN chown -R application:application /usr/src/app
USER application

ENTRYPOINT [ "./advance-server/advance-server" ]
