#include "App.h"
#include "TinyTimber.h"
#include "canTinyTimber.h"
#include "sciTinyTimber.h"
// standard lib
#include "stdlib.h"
#include "string.h"

const char flush = 'F';
const char delimiter = 'e';

extern App app;
extern Can can0;
extern Serial sci0;
extern void DUMPD(int v);

void receiver(App *self, int c)
{
  CANMsg msg;
  CAN_RECEIVE(&can0, &msg);
  SCI_WRITE(&sci0, "Can msg received: ");
  SCI_WRITE(&sci0, msg.buff);
}

void int_to_str(int n, char *str)
{
  int i = 0, is_negative = 0;
  if (n == 0)
  {
    str[i++] = '0';
    str[i] = '\0';
    return;
  }
  if (n < 0)
  {
    is_negative = 1;
    n = -n;
  }
  while (n != 0)
  {
    str[i++] = (n % 10) + '0';
    n = n / 10;
  }
  if (is_negative)
    str[i++] = '-';
  str[i] = '\0';
  // Reverse the string
  for (int j = 0; j < i / 2; j++)
  {
    char temp = str[j];
    str[j] = str[i - j - 1];
    str[i - j - 1] = temp;
  }
}

void debugNumber(App *self)
{
  SCI_WRITE(&sci0, "\nArray: ");
  for (int i = 0; i < 3; i++)
  {
    DUMPD(self->history[i]);
    SCI_WRITE(&sci0, " ");
  }
  SCI_WRITE(&sci0, "\n");
}

int calculateMedian(App *self)
{
  int a = self->history[0];
  int b = self->history[1];
  int c = self->history[2];
  int median;

  if ((a <= b && b <= c) || (c <= b && b <= a))
    median = b;
  else if ((b <= a && a <= c) || (c <= a && a <= b))
    median = a;
  else
    median = c;

  return median;
}

void reader(App *self, int c)
{
  // first character
  SCI_WRITECHAR(&sci0, c);

  if (c == 'F')
  {
    // debugNumber(self);
    SCI_WRITE(&sci0, "\nRcv: 'F'\n");
    for (int i = 0; i < 3; i++)
      self->history[i] = 0;
    self->num_pos = 0;
    SCI_WRITE(&sci0, "The 3-history has been erased\n");
    return;
  }

  // terminator logic \n, \r or 'e'
  if (c == '\n' || c == '\r' || c == 'e')
  {
    self->number[self->num_pos] = '\0';
    int value = atoi(self->number);

    // for history count
    self->history[self->count % 3] = value;
    self->count++;

    // math time
    int sum = self->history[0] + self->history[1] + self->history[2];
    int median = calculateMedian(self);

    // to string
    char s_val[12], s_sum[12], s_med[12];
    int_to_str(value, s_val);
    int_to_str(sum, s_sum);
    int_to_str(median, s_med);

    SCI_WRITE(&sci0, "\nEntered: ");
    SCI_WRITE(&sci0, s_val);
    SCI_WRITE(&sci0, " | Sum: ");
    SCI_WRITE(&sci0, s_sum);
    SCI_WRITE(&sci0, " | Median: ");
    SCI_WRITE(&sci0, s_med);
    SCI_WRITE(&sci0, "\n");

    self->num_pos = 0;
    self->pos = 0;
  }
  else if (self->pos < 1)
  {
    self->buffer[0] = (char)c;
    self->buffer[1] = '\0';

    SCI_WRITE(&sci0, "\nRcv: '");
    SCI_WRITE(&sci0, self->buffer);
    SCI_WRITE(&sci0, "'\n");

    if (self->num_pos < 63)
    {
      self->number[self->num_pos++] = (char)c;
    }

    self->pos = 0;
  }
  else
  {
    SCI_WRITE(&sci0, "\nError: Only 1 char allowed!\n");
  }
}

void startApp(App *self, int arg)
{
  // CANMsg msg;

  // init components: CAN, SCI
  CAN_INIT(&can0);
  SCI_INIT(&sci0);
  SCI_WRITE(&sci0, "Hello, if this code here, the program is setup correctly,... \n");

  // call the method
  // also test if it can output 'A'
  //
}

int main()
{
  INSTALL(&sci0, sci_interrupt, SCI_IRQ0);
  INSTALL(&can0, can_interrupt, CAN_IRQ0);
  TINYTIMBER(&app, startApp, 0);
  return 0;
}
