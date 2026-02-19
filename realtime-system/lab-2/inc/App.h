#ifndef _APP_H
#define _APP_H

#include "TinyTimber.h"

typedef struct {
  Object super;
  char key[3];
  int count;
} App;

#define initApp()                                                              \
  { initObject(), {0}, 0 }

void reader(App *, int);
void receiver(App *, int);
void startApp(App *, int);

#endif
