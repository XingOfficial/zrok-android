#include <jni.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>
#include <atomic>
#include <cstring>
#include <string>
#include <thread>
static std::atomic<int> g_lfd{-1};
static std::atomic<bool> g_run{false};
static bool send_fd(int sock, int fd) {
    char buf[1] = {'F'};
    struct iovec iov { buf, 1 };
    char cbuf[CMSG_SPACE(sizeof(int))];
    memset(cbuf, 0, sizeof cbuf);
    msghdr msg{};
    msg.msg_iov = &iov;
    msg.msg_iovlen = 1;
    msg.msg_control = cbuf;
    msg.msg_controllen = sizeof cbuf;
    cmsghdr* cm = CMSG_FIRSTHDR(&msg);
    cm->cmsg_level = SOL_SOCKET;
    cm->cmsg_type = SCM_RIGHTS;
    cm->cmsg_len = CMSG_LEN(sizeof(int));
    memcpy(CMSG_DATA(cm), &fd, sizeof(int));
    return sendmsg(sock, &msg, 0) >= 0;
}
static void listen_loop(int port, std::string sockName) {
    int lfd = socket(AF_INET, SOCK_STREAM, 0);
    int one = 1;
    setsockopt(lfd, SOL_SOCKET, SO_REUSEADDR, &one, sizeof one);
    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    addr.sin_port = htons(port);
    if (bind(lfd, (sockaddr*)&addr, sizeof addr) < 0 || listen(lfd, 16) < 0) {
        close(lfd);
        g_run = false;
        return;
    }
    g_lfd = lfd;
    while (g_run) {
        int cfd = accept(lfd, nullptr, nullptr);
        if (cfd < 0) break;
        int ufd = socket(AF_UNIX, SOCK_STREAM, 0);
        sockaddr_un sa{};
        sa.sun_family = AF_UNIX;
        sa.sun_path[0] = '\0';
        strncpy(sa.sun_path + 1, sockName.c_str(), sizeof(sa.sun_path) - 2);
        socklen_t slen = offsetof(sockaddr_un, sun_path) + 1 + sockName.size();
        if (connect(ufd, (sockaddr*)&sa, slen) == 0) {
            send_fd(ufd, cfd);
        }
        close(ufd);
        close(cfd);
    }
}
extern "C" JNIEXPORT jboolean JNICALL
Java_cn_xing_zrok_Fwd_nativeStart(JNIEnv* env, jclass, jint port, jstring sockNameJ) {
    if (g_run) return JNI_TRUE;
    const char* s = env->GetStringUTFChars(sockNameJ, nullptr);
    std::string sockName(s);
    env->ReleaseStringUTFChars(sockNameJ, s);
    g_run = true;
    std::thread(listen_loop, (int)port, sockName).detach();
    return JNI_TRUE;
}
extern "C" JNIEXPORT void JNICALL
Java_cn_xing_zrok_Fwd_nativeStop(JNIEnv*, jclass) {
    g_run = false;
    int fd = g_lfd.exchange(-1);
    if (fd >= 0) {
        shutdown(fd, SHUT_RDWR);
        close(fd);
    }
}
