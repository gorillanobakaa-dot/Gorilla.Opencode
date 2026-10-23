/* Exports for the binary_inspect fixtures: two functions, one data object. */
#if defined(_WIN32)
#define EXPORT __declspec(dllexport)
#else
#define EXPORT __attribute__((visibility("default")))
#endif
EXPORT int gorilla_export_add(int a, int b) { return a + b; }
EXPORT int gorilla_export_mul(int a, int b) { return a * b; }
EXPORT int gorilla_export_data = 42;
static int gorilla_hidden(int x) { return x - 1; }
int gorilla_use(int x) { return gorilla_hidden(x); }
