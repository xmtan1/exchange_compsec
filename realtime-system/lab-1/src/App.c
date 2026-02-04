#include "App.h"
#include "TinyTimber.h"
#include "canTinyTimber.h"
#include "sciTinyTimber.h"

extern App app;
extern Can can0;
extern Serial sci0;

void receiver(App *self, int unused)
{
  CANMsg msg;
  CAN_RECEIVE(&can0, &msg);
  SCI_WRITE(&sci0, "Can msg received: ");
  SCI_WRITE(&sci0, msg.buff);
}

void reader(App *self, int c)
{
  // modified to accept a character array
  // SCI_WRITE(&sci0, "Rcv: \'");
  // SCI_WRITECHAR(&sci0, c);
  // SCI_WRITE(&sci0, "\'\n");

  // for a single character
  SCI_WRITECHAR(&sci0, c);

  // check for newline character
  if (c == '\n' || c == '\r')
  {
    // adding end character
    self->buffer[self->pos] = '\0';

    // send string
    SCI_WRITE(&sci0, "\nProcessing string: ");
    SCI_WRITE(&sci0, self->buffer);
    SCI_WRITE(&sci0, "\n");

    // reset upon compelete
    self->pos = 0;
  }
  else if (self->pos < 63)
  {
    self->buffer[self->pos++] = (char)c;
  }
}

void startApp(App *self, int arg)
{
  CANMsg msg;

  // init components: CAN, SCI
  CAN_INIT(&can0);
  SCI_INIT(&sci0);
  SCI_WRITE(&sci0, "Hello, if this code here, the program is setup correctly,... \n");

  // msg.msgId = 1;
  // msg.nodeId = 1;
  // msg.length = 6;
  // msg.buff[0] = 'H';
  // msg.buff[1] = 'e';
  // msg.buff[2] = 'l';
  // msg.buff[3] = 'l';
  // msg.buff[4] = 'o';
  // msg.buff[5] = 0;
  // CAN_SEND(&can0, &msg);

  // call the method
  // also test if it can output 'A'
  reader(self, 'A');
}

void writeCharater(App *self, int c)
{
  // simply write 1 character, not display anything
  SCI_WRITECHAR(&sci0, c);
}

int main()
{
  INSTALL(&sci0, sci_interrupt, SCI_IRQ0);
  INSTALL(&can0, can_interrupt, CAN_IRQ0);
  TINYTIMBER(&app, startApp, 0);
  return 0;
}
