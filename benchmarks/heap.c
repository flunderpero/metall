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

// Floyd's "bounce": bubble the hole at index 0 down to a leaf always following the
// larger child (one compare per level), then sift `v` back up from the leaf. This is
// the classic sift-down with ~half the comparisons of the naive variant, which is what
// makes pop and replace cheap on a steady-state heap.
static inline void sift_down(uint64_t *a, size_t n, uint64_t v) {
    size_t hole = 0;
    size_t child;
    while ((child = 2 * hole + 1) < n) {
        if (child + 1 < n && a[child + 1] > a[child]) child++;
        a[hole] = a[child];
        hole = child;
    }
    while (hole > 0) {
        size_t parent = (hole - 1) / 2;
        if (a[parent] >= v) break;
        a[hole] = a[parent];
        hole = parent;
    }
    a[hole] = v;
}

static inline void heap_push(uint64_t *a, size_t *n, uint64_t v) {
    size_t hole = (*n)++;
    while (hole > 0) {
        size_t parent = (hole - 1) / 2;
        if (a[parent] >= v) break;
        a[hole] = a[parent];
        hole = parent;
    }
    a[hole] = v;
}

static inline uint64_t heap_pop(uint64_t *a, size_t *n) {
    uint64_t top = a[0];
    uint64_t last = a[--(*n)];
    if (*n > 0) sift_down(a, *n, last);
    return top;
}

// Pop the max and insert `v` in a single sift-down. Observably identical to pop()+push()
// (same value returned, same resulting multiset) but does one bounce instead of two sifts.
static inline uint64_t heap_replace(uint64_t *a, size_t n, uint64_t v) {
    uint64_t top = a[0];
    sift_down(a, n, v);
    return top;
}

int main(int argc, char **argv) {
    const char *mode = argc > 1 ? argv[1] : "sort";
    // Read N opaquely so the optimizer cannot precompute the whole run.
    const char *env_n = getenv("HEAP_N");
    const uint64_t N = env_n ? strtoull(env_n, NULL, 10) : 10000000ULL;
    const size_t K = 1024;

    if (!strcmp(mode, "sort")) {
        uint64_t *a = (uint64_t *)malloc(N * sizeof(uint64_t));
        size_t n = 0;
        for (uint64_t i = 0; i < N; i++) heap_push(a, &n, hash(i));
        uint64_t acc = 0;
        while (n > 0) acc = acc * 1000003 + heap_pop(a, &n);
        printf("sort: %llu\n", (unsigned long long)acc);
        free(a);
    } else if (!strcmp(mode, "churn")) {
        const uint64_t M = N * 3;
        uint64_t *a = (uint64_t *)malloc((K + 1) * sizeof(uint64_t));
        size_t n = 0;
        uint64_t acc = 0;
        for (uint64_t i = 0; i < M; i++) {
            heap_push(a, &n, hash(i));
            if (n > K) acc = acc * 1000003 + heap_pop(a, &n);
        }
        while (n > 0) acc = acc * 1000003 + heap_pop(a, &n);
        printf("churn: %llu\n", (unsigned long long)acc);
        free(a);
    } else if (!strcmp(mode, "pushpop")) {
        const uint64_t M = N * 3;
        uint64_t *a = (uint64_t *)malloc(K * sizeof(uint64_t));
        size_t n = 0;
        for (uint64_t i = 0; i < K; i++) heap_push(a, &n, hash(i));
        uint64_t acc = 0;
        for (uint64_t i = 0; i < M; i++) {
            acc = acc * 1000003 + heap_replace(a, n, hash(K + i));
        }
        while (n > 0) acc = acc * 1000003 + heap_pop(a, &n);
        printf("pushpop: %llu\n", (unsigned long long)acc);
        free(a);
    } else {
        fprintf(stderr, "Unknown mode: %s\n", mode);
        return 1;
    }
    return 0;
}
