#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

#define BLACK_BOX(x) __asm__ volatile("" : "+r" (x))

static inline uint64_t hash(uint64_t x) {
    // This forces clang to not optimize the first part of the calculations away.
    // It might feel like cheating by handcuffing C but we don't want to test how
    // good clang can optimize the test-data.
    BLACK_BOX(x);
    x ^= x >> 30;
    x *= 0xbf58476d1ce4e5b9ULL;
    x ^= x >> 27;
    x *= 0x94d049bb133111ebULL;
    x ^= x >> 31;
    return x;
}

static inline int is_space(uint8_t b) {
    return b == ' ' || (b >= 9 && b <= 13);
}

int main(int argc, char **argv) {
    const char *mode = argc > 1 ? argv[1] : "byte";
    const size_t N = 500000000;
    const uint8_t NEEDLE = 7;

    uint8_t *data = (uint8_t *)malloc(N);
    if (!data) return 1;
    for (size_t i = 0; i < N; i++) {
        data[i] = (uint8_t)(hash((uint64_t)i) & 0xff);
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
