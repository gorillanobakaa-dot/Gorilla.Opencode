#include <stdio.h>
static const unsigned short gorilla_wide[] = {'G','O','R','I','L','L','A','_','W','I','D','E','_','1','6',0};
int gorilla_marker(int x) { return x * 7; }
int main(int argc, char **argv) {
    (void)argv;
    puts("GORILLA_FIXTURE_ASCII");
    return gorilla_marker(argc) + (int)gorilla_wide[argc & 7];
}
