// Demonstrates calling `export`ed Metall functions from C.
// See examples/export.met and `just examples` for the build recipe.
#include <stdio.h>

#include "export.h"

int main(void) {
    printf("metall_add(3, 4)   = %lld\n", (long long)metall_add(3, 4));
    printf("metall_fib(10)     = %lld\n", (long long)metall_fib(10));
    printf("metall_is_even(7)  = %s\n", metall_is_even(7) ? "true" : "false");
    printf("metall_is_even(8)  = %s\n", metall_is_even(8) ? "true" : "false");
    return 0;
}
