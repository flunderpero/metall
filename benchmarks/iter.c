#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <stdbool.h>

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

int main(int argc, char **argv) {
    const char *mode = argc > 1 ? argv[1] : "fold";
    const uint64_t N = 500000000ULL;

    if (!strcmp(mode, "fold")) {
        uint64_t acc = 0;
        for (uint64_t i = 0; i < N; i++) {
            uint64_t h = hash(i);
            if (h % 3 == 0) continue;
            acc += h * 17 + 42;
        }
        printf("fold: %llu\n", (unsigned long long)acc);
    } else if (!strcmp(mode, "count")) {
        uint64_t cnt = 0;
        for (uint64_t i = 0; i < N; i++) {
            uint64_t h = hash(i);
            if (h % 3 == 0) continue;
            cnt++;
        }
        printf("count: %llu\n", (unsigned long long)cnt);
    } else if (!strcmp(mode, "all")) {
        bool result = true;
        for (uint64_t i = 0; i < N; i++) {
            uint64_t h = hash(i);
            if (h % 3 == 0) continue;
            uint64_t y = h * 17 + 42;
            if (y >= UINT64_MAX) { result = false; break; }
        }
        printf("all: %s\n", result ? "true" : "false");
    } else if (!strcmp(mode, "any")) {
        uint64_t target = hash(499999990ULL) * 17 + 42;
        bool result = false;
        for (uint64_t i = 0; i < N; i++) {
            uint64_t h = hash(i);
            if (h % 3 == 0) continue;
            uint64_t y = h * 17 + 42;
            if (y == target) { result = true; break; }
        }
        printf("any: %s\n", result ? "true" : "false");
    } else if (!strcmp(mode, "find")) {
        uint64_t target = hash(499999990ULL) * 17 + 42;
        uint64_t found = 0;
        bool have = false;
        for (uint64_t i = 0; i < N; i++) {
            uint64_t h = hash(i);
            if (h % 3 == 0) continue;
            uint64_t y = h * 17 + 42;
            if (y == target) { found = y; have = true; break; }
        }
        if (have) printf("find: %llu\n", (unsigned long long)found);
        else printf("find: None\n");
    } else if (!strcmp(mode, "take")) {
        const uint64_t TAKE = 400000000ULL;
        uint64_t acc = 0;
        uint64_t produced = 0;
        for (uint64_t i = 0; i < N && produced < TAKE; i++) {
            uint64_t h = hash(i);
            if (h % 3 == 0) continue;
            acc += h * 17 + 42;
            produced++;
        }
        printf("take: %llu\n", (unsigned long long)acc);
    } else if (!strcmp(mode, "take_while")) {
        uint64_t threshold = hash(499999990ULL) * 17 + 42;
        uint64_t acc = 0;
        for (uint64_t i = 0; i < N; i++) {
            uint64_t h = hash(i);
            if (h % 3 == 0) continue;
            uint64_t y = h * 17 + 42;
            if (y == threshold) break;
            acc += y;
        }
        printf("take_while: %llu\n", (unsigned long long)acc);
    } else if (!strcmp(mode, "collect")) {
        size_t cap = 1024;
        uint64_t *buf = (uint64_t *)malloc(cap * sizeof(uint64_t));
        size_t len = 0;
        for (uint64_t i = 0; i < N; i++) {
            uint64_t h = hash(i);
            if (h % 3 == 0) continue;
            uint64_t y = h * 17 + 42;
            if (len == cap) {
                cap *= 2;
                buf = (uint64_t *)realloc(buf, cap * sizeof(uint64_t));
            }
            buf[len++] = y;
        }
        printf("collect: %zu\n", len);
        free(buf);
    } else {
        fprintf(stderr, "Unknown mode: %s\n", mode);
        return 1;
    }
    return 0;
}
