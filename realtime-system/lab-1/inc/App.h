#ifndef _APP_H
#define _APP_H

#include "TinyTimber.h"

typedef struct {
  Object super;
  char buffer[64];
  int pos;
} App;

#define initApp()                                                              \
  { initObject(), {0}, 0 }

void reader(App *, int);
void receiver(App *, int);
void startApp(App *, int);

#endif
