#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

static inline uint64_t hash(uint64_t x) {
    x ^= x >> 30;
    x *= 0xbf58476d1ce4e5b9ULL;
    x ^= x >> 27;
    x *= 0x94d049bb133111ebULL;
    x ^= x >> 31;
    return x;
}

int main(int argc, char **argv) {
    const char *mode = argc > 1 ? argv[1] : "grow";
    // Read N opaquely so the optimizer cannot precompute the whole run.
    const char *env_n = getenv("LIST_N");
    const uint64_t N = env_n ? strtoull(env_n, NULL, 10) : 10000000ULL;

    if (!strcmp(mode, "grow")) {
        const int rounds = 100;
        uint64_t acc = 0;
        for (int r = 0; r < rounds; r++) {
            size_t cap = 16, len = 0;
            uint64_t *a = (uint64_t *)malloc(cap * sizeof(uint64_t));
            for (uint64_t i = 0; i < N; i++) {
                uint64_t v = hash(i);
                if (len == cap) { cap *= 2; a = (uint64_t *)realloc(a, cap * sizeof(uint64_t)); }
                a[len++] = v;
                acc += v;
            }
            free(a);
        }
        printf("grow: %llu\n", (unsigned long long)acc);
    } else if (!strcmp(mode, "seq")) {
        const int passes = 1000;
        uint64_t *a = (uint64_t *)malloc(N * sizeof(uint64_t));
        for (uint64_t i = 0; i < N; i++) a[i] = hash(i);
        uint64_t acc = 0;
        for (int p = 0; p < passes; p++)
            for (uint64_t i = 0; i < N; i++) acc += a[i];
        printf("seq: %llu\n", (unsigned long long)acc);
        free(a);
    } else if (!strcmp(mode, "random")) {
        const uint64_t ops = N * 24;
        uint64_t *a = (uint64_t *)malloc(N * sizeof(uint64_t));
        for (uint64_t i = 0; i < N; i++) a[i] = hash(i);
        uint64_t acc = 0;
        for (uint64_t i = 0; i < ops; i++) {
            uint64_t idx = hash(i) % N;
            acc += a[idx];
        }
        printf("random: %llu\n", (unsigned long long)acc);
        free(a);
    } else if (!strcmp(mode, "scatter")) {
        const uint64_t ops = N * 23;
        uint64_t *a = (uint64_t *)malloc(N * sizeof(uint64_t));
        for (uint64_t i = 0; i < N; i++) a[i] = hash(i);
        for (uint64_t i = 0; i < ops; i++) {
            uint64_t idx = hash(i) % N;
            a[idx] += hash(i);
        }
        uint64_t acc = 0;
        for (uint64_t i = 0; i < N; i++) acc += a[i];
        printf("scatter: %llu\n", (unsigned long long)acc);
        free(a);
    } else if (!strcmp(mode, "drain")) {
        const int rounds = 200;
        uint64_t *a = (uint64_t *)malloc(N * sizeof(uint64_t));
        uint64_t acc = 0;
        for (int r = 0; r < rounds; r++) {
            size_t len = 0;
            for (uint64_t i = 0; i < N; i++) a[len++] = hash(i);
            while (len > 0) acc += a[--len];
        }
        printf("drain: %llu\n", (unsigned long long)acc);
        free(a);
    } else {
        fprintf(stderr, "Unknown mode: %s\n", mode);
        return 1;
    }
    return 0;
}
