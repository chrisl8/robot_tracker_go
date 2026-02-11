// Copyright (C) 2017-2021 Vorpal Robotics, LLC.
// Modified in 2026 by Christen Lofland specifically for use in a robotics project
// This version is missing important pieces, such as all of the SD Card functions
// which I removed to make it build/compile without extra libraries
// and has been augmented with code specific to my requirements.
// You can obtain the original code at:
// https://vorpalrobotics.com/wiki/index.php?title=Vorpal_Hexapod_Source_Files
// https://github.com/vorpalrobotics/VorpalHexapod
// https://www.dropbox.com/sh/0stxwsw918kfwa3/AAD4RSyTBRpRV1i8guklWj8na?dl=0

const char *Version = "#GV3r1c-Chris10-0.01"; // this has been modified for this codebase

// This is the code that runs on the Gamepad in the Vorpal The Hexapod project.

//////////// For more information:
// Main website: http://www.vorpalrobotics.com
// Store (for parts and kits): http://store.vorpalrobotics.com
// Wiki entry for this project: http://vorpalrobotics.com/wiki/index.php?title=Vorpal_The_Hexapod
// Wiki information on our other projecs: http://www.vorpalrobotics.com/wiki

///////////  License:
// This work is licensed under the Creative Commons Attribution-NonCommercial-ShareAlike 4.0 International License.
// To view a copy of this license, visit http://creativecommons.org/licenses/by-nc-sa/4.0/.
// Attribution for derivations of this work should be made to: Vorpal Robotics, LLC
//
// You may use this work for noncommercial purposes without cost.
// For information on licensing this work for commercial purposes, please send email to support@vorpalrobotics.com

//////////// Required IDE Version and libraries:
// This program requires several built-in Arduino IDE libraries: SPI, wire, SoftwareSerial

//////////// Processor notes:
// The processor we use on the gamepad is the Arduino Nano ATMEGA328p running at 16 mHz. It would probably work
// on other processors that have at least as much RAM and program memory as the Nano. It's a good idea to use
// a Nano with an FT232 usb serial IO chip rather than the cheaper CH34x chip, because the FT232 will work much more seamlessly
// with Mac computers for use with the Scratch Features. If you don't care about using Scratch on Mac then it doesn't matter.
// Note that on some processors the SPI lines have different pin assignments than the nano so you may need to change a few things.

int debugmode = 0; // Set to 1 to get more debug messages. Warning: this may make Scratch unstable so don't leave it on.

#define DEBUG_BUTTONS 0 // Set to 1 to help debug button issues.

#define DPADDEBUG 0 // Set this to 1 and it prints out dpad raw values. Useful if you have a nonstandard dpad that outputs values
                    // that are atypical. The program right now supports the two most common dpad codings. Holding
                    // down the special (topmost) dpad button during boot will auto-detect and set up EEPROM to
                    // remember what kind of dpad you have.

#include <SoftwareSerial.h>
#include <EEPROM.h>

//////////////////////////////////////////
// Gamepad layout, using variable names used in this code:
//
//                       W
// M0  M1  M2  M3
// M4  M5  M6  M7        F
// M8  M9  M10 M11     L   R
// M12 M13 M14 M15       B
//
//  The buttons on the left side are a 4x4 matrix, connected
// to digital io ports 6-9 (columns) and 2-5 (rows)
// the right side buttons in Dpad configuration are a set of
// resistive buttons and are on A1. Only one of those can be
// detected at a time.
//
// The buttons on the right are used to control motion. The buttons on
// the left signal the "mode" you are in. After pressing one button on the
// left, you stay in that mode until a different mode button is pressed. You
// don't have to keep holding them.
//
// A "long tap" on a mode button uses the scratch recording saved to that button,
// if there is one. If there isn't one, the long tap just does the default action
// (same as short tap).
//
// The rows are marked on the gamepad: W, D, F, R for "walk", "dance", "fight", "record"
// respectively.  See wiki documentation for the W, D, F modes.
//
// Fourth row (M13-M16) are for recording motions. These Buttons leave you
// in the same movement mode, they are independent of movement mode.
// M13: Record/Stop
// M14: Play/Pause
// M15: Rewind to start
// M16: Erase. This has to be held for 3 seconds, after which an extremely annoying
//             high pitched beep indicates the erase has occured.
//
// While recording the internal beeper makes a high pitched short chirp
// every second to remind you. Because this is recording to an SD card,
// and it only takes 130 bytes for 1 second of recording, you have
// tons of recording time. We internally limit it to about 1 hour
// just so people don't forget they have record on and fill up their card.
//
// While playing back the internal beeper will make a high pitched beep
// every second.  When that stops happening the recording is finished.
//
// If you record and stop, then record again, you will be adding on to the
// prior recording unless you rewind first.

// Each movement mode (rows W, D, F) transmits the mode characters, for example W1 or D3, followed by the
// DPAD characters: l, r, f, b, w, s.  These stand for left, right, forward, backward, weapon, and stop
// and they correspond to dpad directions (weapon is the extra dpad button, stop is the lack of any button being pressed).

SoftwareSerial BlueTooth(A5, A4); // connect bluetooth module Tx=A5=Yellow wire Rx=A4=Green Wire
                                  // although not intuitive, the analog ports function just fine as digital ports for this purpose
                                  // and in order to keep the wiring simple for the 4x4 button matrix as well as the SPI lines needed
                                  // for the SD card, it was necessary to use analog ports for the bluetooth module.

// Matrix button definitions
// Top row
#define WALK_1 0
#define WALK_2 1
#define WALK_3 2
#define WALK_4 3
// Second row
#define DANCE_1 4
#define DANCE_2 5
#define DANCE_3 6
#define DANCE_4 7
// Third Row
#define FIGHT_1 8
#define FIGHT_2 9
#define FIGHT_3 10
#define FIGHT_4 11
// Bottom Row
#define REC_RECORD 12
#define REC_PLAY 13
#define REC_REWIND 14
#define REC_ERASE 15

// Dpad styles. Some dpads use different output ranges for buttons
#define STDDPADSTYLE 0
#define ALTDPADSTYLE 1
#define NUMDPADSTYLES 2

byte DpadStyle = STDDPADSTYLE;

// Pin definitions

#define DpadPin A1
#define VCCA1 A2        // we play a trick to power the dpad buttons, use adjacent unusued analog ports for power
#define GNDA1 A3        // Yes, you can make an analog port into a digital output!
#define SDCHIPSELECT 10 // chip select pin for the SD card reader

// definitions to decode the 4x4 button matrix

#define MATRIX_ROW_START 6
#define MATRIX_COL_START 2
#define MATRIX_NROW 4
#define MATRIX_NCOL 4

unsigned long suppressButtonsUntil = 0; // default is not to suppress until we see serial data

byte TrimMode = 0;
byte verbose = 0;
char ModeChars[] = {'W', 'D', 'F', 'R', 'X', 'Y', 'Z'};
char SubmodeChars[] = {'1', '2', '3', '4'};

char CurCmd = ModeChars[0];       // default is Walk mode
char CurSubCmd = SubmodeChars[0]; // default is primary submode
char CurDpad = 's';               // default is stop
unsigned int BeepFreq = 0;        // frequency of next beep command, 0 means no beep, should be range 50 to 2000 otherwise
unsigned int BeepDur = 0;         // duration of next beep command, 0 means no beep, should be in range 1 to 30000 otherwise
                                  // although if you go too short, like less than 30, you'll hardly hear anything

byte HC05_pad = 0; // if 1, we will pad out hc05 transmissions to 230 bytes to force buffer flush for some recent hc05 modules
                   // this can be changed to 1 by holding down D1 while booting, or turned on (set 1) by
                   // holding down D2 while booting
                   // the value is saved in EEPROM and read at boot in setup().
                   // If the robot code sees lots of nulls in between packets it will automatically
                   // use padding on return protocol (such as sensor reports for scratch)

void println()
{
  Serial.println("");
}

void debug(const char *s)
{
  if (verbose || suppressButtonsUntil >= millis())
  {
    Serial.print(s);
  }
}

void debug(int s)
{
  if (verbose || suppressButtonsUntil >= millis())
  {
    Serial.print(s);
  }
}

void debug(long s)
{
  if (verbose || suppressButtonsUntil >= millis())
  {
    Serial.print(s);
  }
}

void debugln()
{
  if (verbose || suppressButtonsUntil >= millis())
  {
    println();
  }
}

// 4x4 Button Matrix function. For now we will just return the first button found, although it is possible to detect
// multiple simultaneous button presses. We might use that in the future to increase the
// effective number of possible controls.

int priorPriorMatrix = -1; // whatever was hit two times ago not including duplicates
int priorMatrix = -1;      // whatever was hit last time not including duplicates
long curMatrixStartTime = 0;
long priorMatrixStartTime = 0;
byte longClick = 0;      // this will be set to 1 if the last matrix button pressed was held a long time
byte priorLongClick = 0; // used to track whether we should beep to indicate new longclick detected.
byte doubleClick = 0;    // are we current in a double-click mode?

#define LONGCLICKMILLIS 500
#define VERYLONGCLICKMILLIS 1000
#define DEBOUNCEMILLIS 80
#define DOUBLECLICKTIME 500

// find out which button is pressed, the first one found is returned
int scanmatrix()
{
  // we will energize row lines then read column lines

  // first set all rows to high impedance mode
  for (int row = 0; row < MATRIX_NROW; row++)
  {
    pinMode(MATRIX_ROW_START + row, INPUT);
  }
  // set all columns to pullup inputs
  for (int col = 0; col < MATRIX_NCOL; col++)
  {
    pinMode(MATRIX_COL_START + col, INPUT_PULLUP);
  }

  // read each row/column combo until we find one that is active
  for (int row = 0; row < MATRIX_NROW; row++)
  {
    // set only the row we're looking at output low
    pinMode(MATRIX_ROW_START + row, OUTPUT);
    digitalWrite(MATRIX_ROW_START + row, LOW);

    for (int col = 0; col < MATRIX_NCOL; col++)
    {
      delayMicroseconds(100);
      if (digitalRead(MATRIX_COL_START + col) == LOW)
      {
        // we found the first pushed button
        if (row < 3)
        {
          CurCmd = ModeChars[row];
          CurSubCmd = SubmodeChars[col];
        }
        int curmatrix = row * MATRIX_NROW + col;
        // Serial.print("START:"); Serial.print(doubleClick); Serial.print(' '); Serial.print(curmatrix); Serial.print(' '); Serial.print(priorMatrix); Serial.print(' '); Serial.print(priorPriorMatrix); Serial.println();
        int clicktime = millis() - curMatrixStartTime;
        if (curmatrix != priorMatrix)
        {

          curMatrixStartTime = millis();
          priorMatrix = curmatrix;
          longClick = priorLongClick = 0;
        }
        else if (clicktime > LONGCLICKMILLIS)
        {
          // User has been holding down the same button continuously for a long time
          if (clicktime > VERYLONGCLICKMILLIS)
          {
            longClick = 2;
          }
          else
          {
            longClick = 1;
          }
        }
        return curmatrix;
      }
    }
    pinMode(MATRIX_ROW_START + row, INPUT); // set back to high impedance
    // delay(1);
  }

  // if we get here no buttons were pushed, return -1 and
  // clear out the timers used for long click detection.
  priorMatrix = -1;
  priorMatrixStartTime = curMatrixStartTime;
  curMatrixStartTime = 0;
  return -1;
}

byte NeedDoubleClickBeep;

void logMatrixClick(short click)
{
  static unsigned long clickTimes[4] = {0, 0, 0};
  static short clickHistory[4] = {-1, -1, -1};

  // clickHistory[0] is the most recent click
  if (click == clickHistory[0])
  {
    // nothing has changed, user is still holding the same button down
    return;
  }

  for (int i = 3; i > 0; i--)
  {
    clickHistory[i] = clickHistory[i - 1];
    clickTimes[i] = clickTimes[i - 1];
  }
  clickHistory[0] = click;
  clickTimes[0] = millis();

  // logic for double click
  // int priorDoubleClick = doubleClick;
  doubleClick = 0;
  unsigned long diff = clickTimes[3] - clickTimes[1];
  int diffi = diff;

  // A double click occurs if we have no click (-1) followed by a button, followed by no click again (-1) followed by the same button as before, and the
  // two clicks of the same button happen in less than DOUBLECLICKTIME. Also, there are no double clicks on the RECORD line of buttons.
  if (clickHistory[1] < REC_RECORD && clickHistory[0] == -1 && clickHistory[2] == -1 && clickHistory[1] == clickHistory[3] && (abs(diffi) < DOUBLECLICKTIME))
  {
    doubleClick = 1;
    CurCmd = ModeChars[(clickHistory[1] / 4) + 4]; // transmit the higher level row character
    setBeep(1000, 100);
  }

  // Set the following #if to 1 to debug double click issues
#if 0
  Serial.print("DCLICK:");Serial.println(doubleClick);Serial.print("CT="); Serial.print(diffi); Serial.print(" CCM=");Serial.write(CurCmd); Serial.println("");
  long now=millis();
  // dump out clickhistory data
  for (int i = 0; i < 4; i++) {
    Serial.print(clickHistory[i]); Serial.print("@"); Serial.println(now-clickTimes[i]);
  }
#endif
}

// This decodes which button is pressed on the DPAD
// There are some DPAD modules that output different values
// and if you get one of those (not sold by Vorpal) you may need
// to test out your dpad to find reasonable values and change
// the values below.

char decode_button(int b)
{

#if DPADDEBUG
  Serial.print("DPAD: ");
  Serial.println(b);
#endif

  // If your DPAD is doing the wrong things for each button, try using the ALTDPAD mode.
  // The gamepad will detect this automatically if you hold down the "Special" (top) DPAD button
  // while booting.

  if (DpadStyle == ALTDPADSTYLE && b > 400 && b < 850)
  { // this is a miscalibrated DPAD, fix it automatically
    // DpadStyle = STDDPADSTYLE;
    // EEPROM.update(0, DpadStyle);
  }

  switch (DpadStyle)
  {
  case ALTDPADSTYLE:
    if (b < 20)
    {
      return 'b'; // backward (bottom button)
    }
    else if (b < 60)
    {
      return 'l'; // left
    }
    else if (b < 130)
    {
      return 'r'; // right
    }
    else if (b < 250)
    {
      return 'f'; // forward (top of diamond)
    }
    else if (b < 800)
    {
      return 'w'; // weapon (very top button) In the documentation this button is called "Special"
                  // but a long time ago we called it "weapon" because it was used in some other
                  // robot projects that were fighting robots. The code still uses "w" since "s" means stop.
    }
    else
    {
      return 's'; // stop (no button is pressed)
    }
    break;

  case STDDPADSTYLE:
  default:
    if (b < 100)
    {
      return 'b'; // backward (bottom button)
    }
    else if (b < 200)
    {
      return 'l'; // left
    }
    else if (b < 400)
    {
      return 'r'; // right
    }
    else if (b < 600)
    {
      return 'f'; // forward (top of diamond)
    }
    else if (b < 850)
    {
      return 'w'; // weapon (very top button) In the documentation this button is called "Special"
                  // but a long time ago we called it "weapon" because it was used in some other
                  // robot projects that were fighting robots. The code still uses "w" since "s" means stop.
    }
    else
    {
      return 's'; // stop (no button is pressed)
    }
  } // end of switch DpadStyle
}

//////////////////////////////////////////////////////////
// BEEP FREQUENCIES
//////////////////////////////////////////////////////////
#define BF_ERROR 100
#define BF_RECORD_CHIRP 1500
#define BF_PLAY_CHIRP 1000
#define BF_PAUSE_CHIRP 2000
#define BF_NOTIFY 500
#define BF_ERASE 2000
#define BF_REWIND 700

//////////////////////////////////////////////////////////
// BEEP DURATIONS
//////////////////////////////////////////////////////////
#define BD_CHIRP 10
#define BD_SHORT 100
#define BD_MED 200
#define BD_LONG 500
#define BD_VERYLONG 2000

//////////////////////////////////////////////////////////
//   RECORD/PLAY FEATURES
/////////////////////////////////////////////////////////

// States for the Scratch Record/play function
// These can't be the same state variable as used for the gamepad REC/PLAY states because you can have
// a scratch recording playing from a long tap button at the same time you've got
// the gamepad REC button working.
#define SREC_STOPPED 0
#define SREC_RECORDING 1
#define SREC_PLAYING 2

// States for the Gamepad record/play function
#define GREC_STOPPED 0
#define GREC_RECORDING 1
#define GREC_PLAYING 2
#define GREC_PAUSED 3
#define GREC_REWINDING 4
#define GREC_ERASING 5

#define REC_MAXLEN (40000)  // we will arbitrarily limit recording time to about 1 hour (forty thousand deci-seconds)
#define REC_FRAMEMILLIS 100 // time between data frames when recording/playing

int GRecState = GREC_STOPPED;        // gamepad recording state
int SRecState = SREC_STOPPED;        // scratch recording state
unsigned long GRecNextEventTime = 0; // next time to record or play a gamepad record/play event
unsigned long SRecNextEventTime = 0; // next time to play a scratch recording event

unsigned long NextTransmitTime = 0; // next time to send a command to the robot
char PlayLoopMode = 0;

void setBeep(int f, int d)
{
  // schedule a beep to go out with the next transmission
  // this is not quite right as there can only be one beep per transmission
  // right now so if two different subsystems wanted to beep at the same time
  // whichever one is scheduled last would win.
  // But because 10 transmits go out per second this seems sufficient, and it's simple
  BeepFreq = f;
  BeepDur = d;
}

long LastRecChirp; // keeps track of the last time we sent a "recording is happening" reminder chirp to the user

int sendbeep(int noheader)
{

  unsigned int beepfreqhigh = highByte(BeepFreq);
  unsigned int beepfreqlow = lowByte(BeepFreq);
  if (!noheader)
  {
    BlueTooth.print("B");
  }
  BlueTooth.write(beepfreqhigh);
  BlueTooth.write(beepfreqlow);

  unsigned int beepdurhigh = highByte(BeepDur);
  unsigned int beepdurlow = lowByte(BeepDur);
  BlueTooth.write(beepdurhigh);
  BlueTooth.write(beepdurlow);

  // return checksum info
  if (noheader)
  {
    return beepfreqhigh + beepfreqlow + beepdurhigh + beepdurlow;
  }
  else
  {
    return 'B' + beepfreqhigh + beepfreqlow + beepdurhigh + beepdurlow;
  }
}

// pad out packet with nulls to force newer hc05 modules to flush

void padwrite(int len)
{
  if (HC05_pad == 0)
  {
    return; // we are not padding
  }
  // we have to add 4 to the length because the length byte of the protocol does not include th V1, the length byte, or the checksum.
  int zero = 0;
  for (int i = len + 4; i < 230; i++)
  {
    BlueTooth.write(zero);
  }
}

// ============ ADDITIONS FOR SERIAL COMMAND INJECTION ============

unsigned long lastSerialCommand = 0;
#define SERIAL_TIMEOUT_MS 500 // Timeout before reverting to gamepad buttons

// Flag to indicate serial is controlling the robot
boolean serialControlling = false;

// Map serial commands to gamepad protocol
void handleSerialCommand(char c)
{
  switch (c)
  {
  // D-Pad
  case 'f':
    CurDpad = 'f';
    serialControlling = true;
    lastSerialCommand = millis();
    break;
  case 'b':
    CurDpad = 'b';
    serialControlling = true;
    lastSerialCommand = millis();
    break;
  case 'l':
    CurDpad = 'l';
    serialControlling = true;
    lastSerialCommand = millis();
    break;
  case 'r':
    CurDpad = 'r';
    serialControlling = true;
    lastSerialCommand = millis();
    break;
  case 'w':
    CurDpad = 'w';
    serialControlling = true;
    lastSerialCommand = millis();
    break;
  case 's':
    CurDpad = 's';
    serialControlling = true;
    lastSerialCommand = millis();
    break;
  // Matrix Row
  case 'W':
    CurCmd = 'W';
    break;
  case 'D':
    CurCmd = 'D';
    break;
  case 'F':
    CurCmd = 'F';
    break;
  // Matrix Column
  case '1':
    CurSubCmd = '1';
    break;
  case '2':
    CurSubCmd = '2';
    break;
  case '3':
    CurSubCmd = '3';
    break;
  case '4':
    CurSubCmd = '4';
    break;
  // Special Features
  case 'd':
    if (debugmode == 1)
    {
      debugmode = 0;
    }
    else
    {
      debugmode = 1;
    }
    break;
  }
}

void handleSerialInput()
{
  while (Serial.available() > 0)
  {
    char c = Serial.read();
    handleSerialCommand(c);
  }

  // Check for serial timeout
  if (serialControlling && (millis() - lastSerialCommand > SERIAL_TIMEOUT_MS))
  {
    serialControlling = false;
  }
}

// ================================================================

//
// Standard setup function for Arduino, run once at program boot
//

void setup()
{
  Serial.begin(9600);

  HC05_pad = EEPROM.read(1);
  if (HC05_pad == 255)
  { // eeprom was never set up
    HC05_pad = 0;
  }
  // see if we're supposed to be in trim mode or card format mode or HC05 pad mode
  int mat = scanmatrix();
  if (mat == WALK_1)
  {
    Serial.println("#trim");
    TrimMode = 1;
  }
  else if (mat == DANCE_1)
  { // no padding of BT packets
    HC05_pad = 0;
    EEPROM.update(1, HC05_pad); // save for future boots
  }
  else if (mat == DANCE_2)
  { // pad BT packets
    HC05_pad = 1;
    EEPROM.update(1, HC05_pad); // save for future boots
  }

  // make a characteristic flashing pattern to indicate the gamepad code is loaded.
  pinMode(13, OUTPUT);
  for (int i = 0; i < 3; i++)
  {
    digitalWrite(13, !digitalRead(13));
    delay(250);
  }
  // after this point you can't flash the led on pin 13 because we're using it for SD card

  BlueTooth.begin(9600);

  pinMode(A0, OUTPUT); // extra ground for additional FTDI port if needed
  digitalWrite(A0, LOW);
  pinMode(VCCA1, OUTPUT);
  pinMode(GNDA1, OUTPUT);
  digitalWrite(GNDA1, LOW);
  digitalWrite(VCCA1, HIGH);
  pinMode(SDCHIPSELECT, OUTPUT);
  digitalWrite(SDCHIPSELECT, HIGH); // chip select for SD card

  // NOTE: My controller was defaulting to ALTDPADSTYLE despite being STDDPADSTYLE so I removed the Dpad selection code that was here.
  // ============ ADDITIONS FOR SERIAL COMMAND INJECTION ============
  Serial.println(Version);
  // ================================================================
}

int priormatrix = -1;
long curmatrixstarttime = 0; // used to detect long tap for play button and erase button

// Scratch integration: If anything appears on the serial input,
// it's probably coming from a scratch program, so simply send it
// out to the robot, unless it appears to be a command to start
// recording onto a gamepad button

// The following are states for the scratch state machine

#define SCR_WAITING_FOR_HEADER 0
#define SCR_WAITING_FOR_HEX_1 1
#define SCR_WAITING_FOR_REC_1 2
#define SCR_WAITING_FOR_LENGTH 3
#define SCR_HEX_TRANSFER 4
#define SCR_REC_COMMAND 5

#define SCR_MAX_COMMAND_LENGTH 80 // we never expect scratch to send more than 16 commands at a time and max command len is 5 bytes.
                                  // 16 commands is enough to send a move individually to every servo, plus a beep, plus a sensor
                                  // request, plus a couple more besides that. At 80 bytes the packet would require about 20 millisec
                                  // to send over bluetooth, which allows plenty of time for the hexapod to send back the sensor
                                  // data before the next transmission. Although the bluetooth hardware is full duplex, the
                                  // softwareserial library currently is not, so we can't be both reading and writing the bluetooth
                                  // module at the same time.

void send_trim(int matrix, int dpad)
{
  int trim = dpad;

  if (matrix >= REC_RECORD)
  {
    // Serial.print("TRIM-MATRIX=");Serial.println(matrix);
    switch (matrix)
    {
    case REC_RECORD:
      if (longClick == 2)
      { // only do this on a very long click
        trim = 'S';
      }
      break;
    case REC_PLAY:
      trim = 'P';
      break;
    case REC_REWIND:
      trim = 'R';
      break;
    case REC_ERASE:
      if (longClick == 2)
      { // only do this on a very long click
        trim = 'E';
      }
      break;
    }
  }
  // send the trim command

  int two = 2;
  BlueTooth.print("V1");
  BlueTooth.write(two);
  BlueTooth.write('T');
  BlueTooth.write(trim);

  unsigned int checksum = two + 'T' + trim;
  checksum = (checksum % 256);
  BlueTooth.write(checksum);
  padwrite(two);
}

void loop()
{
  // ============ ADD SERIAL COMMAND HANDLING ============
  handleSerialInput();
  if (!serialControlling)
  {
    // ====================================================

    int matrix = scanmatrix();

    logMatrixClick(matrix); // does analysis to determine double click and long click
    CurDpad = decode_button(analogRead(DpadPin));

    if (debugmode && matrix != -1)
    {
      Serial.print("#MA:LC=");
      Serial.print(longClick);
      Serial.print("m:");
      Serial.println(matrix);
    }

    if (debugmode && CurDpad != 's')
    {
      // Serial.print("#DP:"); Serial.println(CurDpad);
    }

    if (TrimMode)
    {
      // special mode where we just transmit the DPAD buttons or the buttons on the record line in a special way.
      // if (CurDpad != 's') { Serial.print("#TRIM:"); Serial.println(CurDpad); }
      send_trim(matrix, CurDpad);
      delay(200);
      return;
    }

    if (priormatrix != matrix)
    {                                // the matrix button pressed has changed from the prior loop iteration
      curmatrixstarttime = millis(); // used to detect long tap
      if (matrix != -1)
      { // -1 means nothing was pressed
        // short beep for button press feedback
        setBeep(200, 50);
      }
    }

    if (longClick && !priorLongClick)
    { // we have detected a long click with no prior long click
      if (matrix < REC_RECORD)
      {                     // don't beep if it's a record button
        setBeep(2000, 100); // high pitch beep tells user they are in long click mode
#if DEBUG_BUTTONS
        Serial.println("#LCL"); // long click
#endif
      }
      priorLongClick = longClick; // keep track of whether we are already long clicking
    }

    // if the robot sends something back to us, print it to the serial port
    // this is likely to be sensor data for scratch or debugging info

    while (BlueTooth.available())
    {
      Serial.write(BlueTooth.read());
    }

    // if we get here we can finally handle the incoming button presses

    // The record/play buttons are treated specially by this switch statement.

    switch (matrix)
    {
    case REC_RECORD:
      // because this button takes on different meanings to avoid bouncing
      // we check to ensure nothing was previously pushed
      longClick = 0;
      if (priormatrix == -1)
      {
        if (GRecState == GREC_RECORDING)
        {
          // if we were already recording, the record button causes a stop
          GRecState = GREC_STOPPED;
          setBeep(BF_NOTIFY, BD_MED);
        }
      } // end of "if (priormatrix == -1)
      break;

    case REC_PLAY:
      longClick = 0;
      break;

    case REC_REWIND:
      break;

    case REC_ERASE:
      break;

    default: // -1 or any W, D, F mode there is nothing to do
      break;
    }

    priormatrix = matrix;
    //
    // The following code handles the Scratch recording to a mode button feature
    //

    if (longClick && (CurCmd == 'W' || CurCmd == 'D' || CurCmd == 'F'))
    {
      // Long click was for recording, which was removed from this code, so it doesn't really have any purpose.
    }
  }
  if (millis() > NextTransmitTime && GRecState != GREC_PLAYING && SRecState != SREC_PLAYING)
  { // don't transmit joystick controls during replay mode!

    // Packet consists of:
    // Byte 0: The letter "V" is used as a header. (Vorpal)
    // Byte 1: The letter "1" which is the version number of the protocol. In the future there may be different gamepad and
    //         robot protocols and this would allow some interoperability between robots and gamepads of different version.
    // Byte 2: L, the length of the data payload in the packet. Right now it is always 8. This number is
    //          only the payload bytes, it does not include the "V", the length, or the checksum
    // Bytes 3 through 3+L-1  The actual data.
    // Byte 3+L The base 256 checksum. The sum of all the payload bytes plus the L byte modulo 256.
    //
    // Right now the
    //

    if (debugmode)
    {
      Serial.print("#S=");
      Serial.print(CurCmd);
      Serial.print(CurSubCmd);
      Serial.println(CurDpad);
    }
    BlueTooth.print("V1"); // Vorpal hexapod radio protocol header version 1
    int eight = 8;
    BlueTooth.write(eight);
    BlueTooth.write(CurCmd);
    BlueTooth.write(CurSubCmd);
    BlueTooth.write(CurDpad);

    unsigned int checksum = sendbeep(0);

    checksum += eight + CurCmd + CurSubCmd + CurDpad;
    checksum = (checksum % 256);
    BlueTooth.write(checksum);
    padwrite(eight);

    setBeep(0, 0); // clear the current beep because it's been sent now

    NextTransmitTime = millis() + REC_FRAMEMILLIS;
  }
}
