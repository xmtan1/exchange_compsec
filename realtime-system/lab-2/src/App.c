#include "App.h"
#include "TinyTimber.h"
#include "canTinyTimber.h"
#include "sciTinyTimber.h"
// standard lib
#include "stdlib.h"
#include "string.h"

extern App app;
extern Can can0;
extern Serial sci0;

const int MIN_INDICATE = -10;
const int MAX_INDICATE = 14;

const int medolies[32] = {0, 2, 4, 0, 0, 2, 4, 0, 4, 5, 7, 4, 5, 7, 7, 9, 7, 5, 4, 0, 7, 9, 7, 5, 4, 0, 0, -5, 0, 0, -5, 0};
const int periods[25] = {2024, 1911, 1803, 1702, 1607, 1516, 1431, 1351, 1275, 1203, 1136, 1072, 1012, 955, 901, 851, 803, 758, 715, 675, 637, 601, 568, 536};

void receiver(App *self, int unused)
{
  CANMsg msg;
  CAN_RECEIVE(&can0, &msg);
  SCI_WRITE(&sci0, "Can msg received: ");
  SCI_WRITE(&sci0, msg.buff);
}

// convert to string (for better printing)
void int_to_str(int n, char *str)
{
  int i = 0, is_negative = 0;
  if (n == 0)
  {
    str[i] = '0';
    i++;
    str[i] = '\0';
  }
  if (n < 0)
  {
    is_negative = 1;
    n = -n;
  }
  while (n != 0)
  {
    str[i] = (n % 10) + '0';
    i++;
    n = n / 10;
  }
  if (is_negative)
  {
    str[i] = '-';
    i++;
  }
  str[i] = '\0';
  for (int j = 0; j < i / 2; j++)
  {
    char tmp = str[j];
    str[j] = str[i - j - 1];
    str[i - j - 1] = tmp;
  }
}

// fetch and print the period array

void print_period_by_key(int x){
  for (int i = 0; i < 32; i++){
    int base = medolies[i];

    int shifted_key = base + x;

    int index = shifted_key - MIN_INDICATE;

    char buffer[12];
    int_to_str(periods[index], buffer);

    SCI_WRITE(&sci0, buffer);
    SCI_WRITE(&sci0, " ");
  }

  SCI_WRITE(&sci0, '\n');
}

void reader(App *self, int c)
{
  // read character logic
  SCI_WRITECHAR(&sci0, c);

  // terminated logic
  if (c == '\n' || c == '\r')
  {
    // put null
    self->key[self->count] = '\0';
    int value = atoi(self->key);

    char input_key[3];
    int_to_str(value, input_key);

    SCI_WRITE(&sci0, "\nKey: ");
    SCI_WRITE(&sci0, input_key);
    SCI_WRITE(&sci0, '\n');

    self->count = 0;

    print_period_by_key(value);
  }
  else if (self->count < 3){
    self->key[self->count++] = (char)c;
  }
}

void startApp(App *self, int arg)
{
  CANMsg msg;

  CAN_INIT(&can0);
  SCI_INIT(&sci0);
  SCI_WRITE(&sci0, "Hello, hello...\n");

  msg.msgId = 1;
  msg.nodeId = 1;
  msg.length = 6;
  msg.buff[0] = 'H';
  msg.buff[1] = 'e';
  msg.buff[2] = 'l';
  msg.buff[3] = 'l';
  msg.buff[4] = 'o';
  msg.buff[5] = 0;
  CAN_SEND(&can0, &msg);
}

int main()
{
  INSTALL(&sci0, sci_interrupt, SCI_IRQ0);
  INSTALL(&can0, can_interrupt, CAN_IRQ0);
  TINYTIMBER(&app, startApp, 0);
  return 0;
}
