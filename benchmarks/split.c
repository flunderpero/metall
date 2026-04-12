#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

static inline int is_space(uint8_t b) {
    return b == ' ' || (b >= 9 && b <= 13);
}

int main(int argc, char **argv) {
    const char *mode = argc > 1 ? argv[1] : "byte";
    const size_t N = 1000000000;
    const uint8_t NEEDLE = 7;

    uint8_t *data = (uint8_t *)malloc(N);
    if (!data) return 1;
    for (size_t i = 0; i < N; i++) {
        data[i] = (uint8_t)(i & 0xff);
    }

    // Number of split pieces = number of separators + 1.
    // The branchless form lets clang autovectorize with NEON.
    size_t seps = 0;
    if (!strcmp(mode, "byte")) {
        for (size_t i = 0; i < N; i++) {
            seps += (data[i] == NEEDLE);
        }
    } else if (!strcmp(mode, "predicate")) {
        for (size_t i = 0; i < N; i++) {
            seps += is_space(data[i]);
        }
    } else {
        fprintf(stderr, "unknown mode: %s\n", mode);
        free(data);
        return 1;
    }
    printf("%zu\n", seps + 1);

    free(data);
    return 0;
}
