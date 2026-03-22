#include <errno.h>
#include <fcntl.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/epoll.h>
#include <sys/socket.h>
#include <unistd.h>

#define PORT 8080
#define MAX_EVENTS 1024
#define BUF_SIZE 4096

static const char RESPONSE[] =
    "HTTP/1.1 200 OK\r\n"
    "Content-Length: 2\r\n"
    "Content-Type: text/plain\r\n"
    "Connection: keep-alive\r\n"
    "\r\n"
    "ok";

static const int RESPONSE_LEN = sizeof(RESPONSE) - 1;

static void set_nonblocking(int fd) {
    int flags = fcntl(fd, F_GETFL, 0);
    fcntl(fd, F_SETFL, flags | O_NONBLOCK);
}

int main(void) {
    signal(SIGPIPE, SIG_IGN);

    int server_fd = socket(AF_INET, SOCK_STREAM, 0);
    if (server_fd < 0) {
        perror("socket");
        return 1;
    }

    int opt = 1;
    setsockopt(server_fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));
    setsockopt(server_fd, SOL_SOCKET, SO_REUSEPORT, &opt, sizeof(opt));
    setsockopt(server_fd, IPPROTO_TCP, TCP_NODELAY, &opt, sizeof(opt));

    struct sockaddr_in addr = {
        .sin_family = AF_INET,
        .sin_port = htons(PORT),
        .sin_addr.s_addr = htonl(INADDR_LOOPBACK),
    };

    if (bind(server_fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
        perror("bind");
        return 1;
    }

    if (listen(server_fd, SOMAXCONN) < 0) {
        perror("listen");
        return 1;
    }

    set_nonblocking(server_fd);

    int epoll_fd = epoll_create1(0);
    struct epoll_event ev = {.events = EPOLLIN, .data.fd = server_fd};
    epoll_ctl(epoll_fd, EPOLL_CTL_ADD, server_fd, &ev);

    struct epoll_event events[MAX_EVENTS];
    char buf[BUF_SIZE];

    fprintf(stderr, "Listening on http://127.0.0.1:%d\n", PORT);

    for (;;) {
        int n = epoll_wait(epoll_fd, events, MAX_EVENTS, -1);
        for (int i = 0; i < n; i++) {
            if (events[i].data.fd == server_fd) {
                /* Accept all pending connections */
                for (;;) {
                    int client_fd = accept(server_fd, NULL, NULL);
                    if (client_fd < 0) {
                        if (errno == EAGAIN || errno == EWOULDBLOCK)
                            break;
                        continue;
                    }
                    set_nonblocking(client_fd);
                    int tcp_opt = 1;
                    setsockopt(client_fd, IPPROTO_TCP, TCP_NODELAY, &tcp_opt, sizeof(tcp_opt));
                    struct epoll_event cev = {.events = EPOLLIN | EPOLLET, .data.fd = client_fd};
                    epoll_ctl(epoll_fd, EPOLL_CTL_ADD, client_fd, &cev);
                }
            } else {
                int fd = events[i].data.fd;
                /* Read all available data and respond to each request */
                for (;;) {
                    ssize_t nread = read(fd, buf, BUF_SIZE);
                    if (nread <= 0) {
                        if (nread == 0 || (errno != EAGAIN && errno != EWOULDBLOCK)) {
                            close(fd);
                        }
                        break;
                    }
                    if (write(fd, RESPONSE, RESPONSE_LEN) < 0) {
                        close(fd);
                        break;
                    }
                }
            }
        }
    }

    close(server_fd);
    close(epoll_fd);
    return 0;
}
