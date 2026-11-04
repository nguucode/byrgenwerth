#include <stdint.h>

// App and windows. Coordinates are global points with the origin at the top left
// of the primary screen, the same as ScreenCaptureKit.
void bw_run(const char *url, int width, int height);
void bw_quit(void);
void bw_main_show(void);
void bw_main_hide(void);
int bw_main_visible(void);
void bw_main_resize(int width, int height);
void bw_main_top(int on);
int bw_overlay_create(const char *url);
void bw_overlay_frame(int id, int x, int y, int w, int h);
void bw_overlay_show(int id, int show);
void bw_overlay_edit(int id, int on);
void bw_overlay_origin(int id, int *x, int *y);

// Screens: writes up to max entries of {left, top, right, bottom, primary}.
int bw_screens(int32_t *out, int max);
int bw_front_fullscreen(int loose);

// Capture and OCR. Returned strings are malloc'd; the caller frees them.
int bw_screen_access(int request);
char *bw_capture(int x, int y, int w, int h, int cw, int ch, uint8_t *rgba);
char *bw_ocr(const uint8_t *rgba, int w, int h, const char *lang);
char *bw_ocr_langs(void);

// Hotkeys, keychain and small native helpers.
int bw_hotkey(int id, int keycode, int mods);
int bw_key_save(const char *key);
char *bw_key_load(void);
char *bw_font_families(void);
void bw_copy(const char *text);
char *bw_choose_file(void);
