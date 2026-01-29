#ifndef _APP_H
#define _APP_H

#include "TinyTimber.h"

typedef struct {
  Object super;
  int example;
} App;

#define initApp()                                                              \
  { initObject(), 1234 }

void reader(App *, int);
void receiver(App *, int);
void startApp(App *, int);

#endif
