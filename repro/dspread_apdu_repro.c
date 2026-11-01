#include <ctype.h>
#include <errno.h>
#include <fcntl.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/select.h>
#include <termios.h>
#include <time.h>
#include <unistd.h>

#include "pos_sdk.h"

#define MAX_APDU_LEN 1024
#define IO_BUF_LEN 4096

#ifdef DSPREAD_ARM_CALLBACKS
/* The ARM archive requires these external callbacks even for NFC transport.
 * Match the Go binding's decimal conversion. RSA is outside this transport
 * repro; abort if an unexpected SDK path attempts to use it. */
void myitoa(int value, char *destination, int base) {
    (void)base;
    sprintf(destination, "%d", value);
}

void RSAPublicBlock(void) {
    fprintf(stderr, "RSA operations are unsupported by this NFC transport repro\n");
    abort();
}
#endif

static void print_hex(const char *label, const unsigned char *data, int len) {
    printf("%s", label);
    for (int i = 0; i < len; i++) {
        printf("%02X", data[i]);
    }
    printf("\n");
}

static int parse_hex(const char *hex, unsigned char *out, int out_cap) {
    int high = -1;
    int len = 0;

    for (const char *p = hex; *p; p++) {
        if (isspace((unsigned char)*p) || *p == ':' || *p == '-') {
            continue;
        }

        int value;
        if (*p >= '0' && *p <= '9') {
            value = *p - '0';
        } else if (*p >= 'a' && *p <= 'f') {
            value = *p - 'a' + 10;
        } else if (*p >= 'A' && *p <= 'F') {
            value = *p - 'A' + 10;
        } else {
            fprintf(stderr, "invalid hex character: %c\n", *p);
            return -1;
        }

        if (high < 0) {
            high = value;
            continue;
        }

        if (len >= out_cap) {
            fprintf(stderr, "hex input too long\n");
            return -1;
        }
        out[len++] = (unsigned char)((high << 4) | value);
        high = -1;
    }

    if (high >= 0) {
        fprintf(stderr, "odd number of hex digits\n");
        return -1;
    }
    return len;
}

static speed_t baud_to_speed(int baud) {
    switch (baud) {
    case 9600:
        return B9600;
    case 19200:
        return B19200;
    case 38400:
        return B38400;
    case 57600:
        return B57600;
    case 115200:
        return B115200;
    case 230400:
        return B230400;
    default:
        return 0;
    }
}

static int open_serial(const char *path, int baud) {
    int fd = open(path, O_RDWR | O_NOCTTY | O_NONBLOCK);
    if (fd < 0) {
        perror("open serial");
        return -1;
    }

    struct termios tty;
    if (tcgetattr(fd, &tty) != 0) {
        perror("tcgetattr");
        close(fd);
        return -1;
    }

    cfmakeraw(&tty);
    tty.c_cflag |= CLOCAL | CREAD;
    tty.c_cflag &= ~CRTSCTS;
    tty.c_cflag &= ~CSTOPB;
    tty.c_cflag &= ~PARENB;
    tty.c_cflag &= ~CSIZE;
    tty.c_cflag |= CS8;
    tty.c_cc[VMIN] = 0;
    tty.c_cc[VTIME] = 0;

    speed_t speed = baud_to_speed(baud);
    if (speed == 0) {
        fprintf(stderr, "unsupported baud rate: %d\n", baud);
        close(fd);
        return -1;
    }
    cfsetispeed(&tty, speed);
    cfsetospeed(&tty, speed);

    if (tcsetattr(fd, TCSANOW, &tty) != 0) {
        perror("tcsetattr");
        close(fd);
        return -1;
    }

    tcflush(fd, TCIOFLUSH);
    return fd;
}

static int monotonic_ms(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (int)(ts.tv_sec * 1000 + ts.tv_nsec / 1000000);
}

static int transmit_frame(int fd, const unsigned char *frame, int frame_len, int timeout_ms) {
    unsigned char buf[IO_BUF_LEN];
    int total = 0;
    int start = monotonic_ms();

    print_hex("write: ", frame, frame_len);

    int written = write(fd, frame, (size_t)frame_len);
    if (written != frame_len) {
        perror("write");
        return -1;
    }
    tcdrain(fd);

    while (monotonic_ms() - start < timeout_ms) {
        fd_set readfds;
        FD_ZERO(&readfds);
        FD_SET(fd, &readfds);

        struct timeval tv;
        tv.tv_sec = 0;
        tv.tv_usec = 100000;

        int ready = select(fd + 1, &readfds, NULL, NULL, &tv);
        if (ready < 0) {
            if (errno == EINTR) {
                continue;
            }
            perror("select");
            return -1;
        }
        if (ready == 0) {
            continue;
        }

        int n = (int)read(fd, buf, sizeof(buf));
        if (n < 0) {
            if (errno == EAGAIN || errno == EWOULDBLOCK || errno == EINTR) {
                continue;
            }
            perror("read");
            return -1;
        }
        if (n == 0) {
            continue;
        }

        total += n;
        print_hex("read-chunk: ", buf, n);
        on_package(buf, n);

        if (is_package_receive_complete()) {
            printf("package-complete: yes, raw-bytes-read=%d\n", total);
            return total;
        }
    }

    fprintf(stderr, "timeout waiting for complete package, raw-bytes-read=%d\n", total);
    return -1;
}

static int query_until_complete(int fd, int serial_timeout_ms) {
    while (getCmdId() == 0x23 || getCmdId() == 0x36) {
        unsigned char out[1024];
        int len = packQueryLatestCmdResult(out);
        if (len < 0) {
            fprintf(stderr, "packQueryLatestCmdResult failed: %d\n", len);
            return -1;
        }
        if (transmit_frame(fd, out, len, serial_timeout_ms) < 0) {
            return -1;
        }
    }
    return 0;
}

static void print_sdk_key(const char *key) {
    unsigned char out[1024];
    int len = get((char *)key, out);
    if (len < 0) {
        printf("%s: get failed (%d)\n", key, len);
        return;
    }
    printf("%s: ", key);
    for (int i = 0; i < len; i++) {
        printf("%02X", out[i]);
    }
    printf("\n");
}

static int poll_mifare(int fd, int serial_timeout_ms) {
    unsigned char out[1024];
    int len = doMifare(0x01, 20, out);
    if (len < 0) {
        fprintf(stderr, "doMifare poll failed: %d\n", len);
        return -1;
    }
    if (transmit_frame(fd, out, len, serial_timeout_ms) < 0) {
        return -1;
    }
    if (query_until_complete(fd, serial_timeout_ms) < 0) {
        return -1;
    }
    printf("cmdid: 0x%02X\n", getCmdId());
    print_sdk_key("mifare_cardUid");
    print_sdk_key("mifare_SAK");
    print_sdk_key("mifare_ATQA");
    return 0;
}

static int power_on_nfc(int fd, unsigned int sdk_timeout, int serial_timeout_ms) {
    unsigned char out[1024];
    int len = powerOnNFC(0x00, sdk_timeout, out);
    if (len < 0) {
        fprintf(stderr, "powerOnNFC failed: %d\n", len);
        return -1;
    }
    if (transmit_frame(fd, out, len, serial_timeout_ms) < 0) {
        return -1;
    }
    if (query_until_complete(fd, serial_timeout_ms) < 0) {
        return -1;
    }
    printf("cmdid: 0x%02X\n", getCmdId());
    print_sdk_key("HasCard");
    print_sdk_key("Atr");
    return 0;
}

static int send_apdu(int fd, const unsigned char *apdu, int apdu_len, unsigned int sdk_timeout, int serial_timeout_ms) {
    unsigned char out[2048];
    int len = sendApduByNFC((unsigned char *)apdu, (unsigned int)apdu_len, sdk_timeout, out);
    if (len < 0) {
        fprintf(stderr, "sendApduByNFC pack failed: %d\n", len);
        return -1;
    }

    print_hex("apdu: ", apdu, apdu_len);
    if (transmit_frame(fd, out, len, serial_timeout_ms) < 0) {
        return -1;
    }
    if (query_until_complete(fd, serial_timeout_ms) < 0) {
        return -1;
    }

    printf("cmdid: 0x%02X\n", getCmdId());
    print_sdk_key("ApduResult");
    print_sdk_key("ApduLen");
    print_sdk_key("ApduEncrpt");
    return 0;
}

static void usage(const char *argv0) {
    fprintf(stderr,
        "usage: %s [-p /dev/ttyACM0] [-b 115200] [-t 60] [-T 3000] [--poweron-nfc] [--stdin-apdu] --apdu HEX [--apdu HEX ...]\n"
            "\n"
            "Pure C Dspread SDK NFC APDU transport repro. It uses only pos_sdk.h and the selected static SDK library.\n"
            "Use `make SDK_LIB=dspreadsdk-for-arm-arch` when building against libs/libdspreadsdk-for-arm-arch.a for ARM device loads.\n"
            "By default it starts with doMifare(0x01), matching the working CR100 DESFire flow.\n"
            "\n"
            "options:\n"
            "  -p PATH       serial device (default: /dev/ttyACM0)\n"
            "  -b BAUD       baud rate (default: 115200)\n"
            "  -t VALUE      raw timeout passed to sendApduByNFC/powerOnNFC (default: 60)\n"
            "  -T MS         serial wait timeout in milliseconds (default: 3000)\n"
            "  --poweron-nfc use powerOnNFC instead of doMifare(0x01) before APDUs\n"
            "  --stdin-apdu  read additional APDU hex lines from stdin after fixed --apdu values\n"
            "  --apdu HEX    APDU bytes to transmit through sendApduByNFC\n",
            argv0);
}

int main(int argc, char **argv) {
    const char *port = "/dev/ttyACM0";
    int baud = 115200;
    unsigned int sdk_timeout = 60;
    int serial_timeout_ms = 3000;
    int use_poweron_nfc = 0;
    int stdin_apdu = 0;
    const char *apdus[128];
    int apdu_count = 0;

    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "-p") == 0 && i + 1 < argc) {
            port = argv[++i];
        } else if (strcmp(argv[i], "-b") == 0 && i + 1 < argc) {
            baud = atoi(argv[++i]);
        } else if (strcmp(argv[i], "-t") == 0 && i + 1 < argc) {
            sdk_timeout = (unsigned int)strtoul(argv[++i], NULL, 0);
        } else if (strcmp(argv[i], "-T") == 0 && i + 1 < argc) {
            serial_timeout_ms = atoi(argv[++i]);
        } else if (strcmp(argv[i], "--poweron-nfc") == 0) {
            use_poweron_nfc = 1;
        } else if (strcmp(argv[i], "--stdin-apdu") == 0) {
            stdin_apdu = 1;
        } else if (strcmp(argv[i], "--apdu") == 0 && i + 1 < argc) {
            if (apdu_count >= (int)(sizeof(apdus) / sizeof(apdus[0]))) {
                fprintf(stderr, "too many APDUs\n");
                return 2;
            }
            apdus[apdu_count++] = argv[++i];
        } else if (strcmp(argv[i], "-h") == 0 || strcmp(argv[i], "--help") == 0) {
            usage(argv[0]);
            return 0;
        } else {
            usage(argv[0]);
            return 2;
        }
    }

    if (apdu_count == 0 && !stdin_apdu) {
        usage(argv[0]);
        return 2;
    }

    int fd = open_serial(port, baud);
    if (fd < 0) {
        return 1;
    }

    int rc = use_poweron_nfc ? power_on_nfc(fd, sdk_timeout, serial_timeout_ms) : poll_mifare(fd, serial_timeout_ms);
    if (rc < 0) {
        close(fd);
        return 1;
    }

    for (int i = 0; i < apdu_count; i++) {
        unsigned char apdu[MAX_APDU_LEN];
        int apdu_len = parse_hex(apdus[i], apdu, sizeof(apdu));
        if (apdu_len < 0) {
            close(fd);
            return 2;
        }
        printf("\n--- APDU %d/%d ---\n", i + 1, apdu_count);
        if (send_apdu(fd, apdu, apdu_len, sdk_timeout, serial_timeout_ms) < 0) {
            close(fd);
            return 1;
        }
    }

    if (stdin_apdu) {
        char line[4096];
        int index = apdu_count + 1;

        fprintf(stderr, "Enter APDU hex lines. Empty line or EOF exits.\n");
        while (fgets(line, sizeof(line), stdin) != NULL) {
            unsigned char apdu[MAX_APDU_LEN];
            int apdu_len;

            if (line[0] == '\n' || line[0] == '\r' || line[0] == 0) {
                break;
            }

            apdu_len = parse_hex(line, apdu, sizeof(apdu));
            if (apdu_len < 0) {
                close(fd);
                return 2;
            }

            printf("\n--- APDU %d/stdin ---\n", index++);
            if (send_apdu(fd, apdu, apdu_len, sdk_timeout, serial_timeout_ms) < 0) {
                close(fd);
                return 1;
            }
        }
    }

    close(fd);
    return 0;
}
