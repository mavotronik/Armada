// UART hub for a 3.3V Arduino Pro Mini (ATmega328P, 8 MHz).
//
// One hardware UART faces the LuckFox. Each cluster node gets a SoftwareSerial
// port. SoftwareSerial can receive on only one port at a time, which matches
// this hub: the master selects a node, talks, then selects the next.
// On an 8 MHz Pro Mini, SoftwareSerial is reliable at 9600. The LuckFox link
// stays at 115200 because that UART is hardware. Do not raise NODE_BAUD unless
// the nodes and this sketch change together.
//
// Set NUM_NODES to 4 or 6. Pins are laid out for six; unused ones stay idle.
//
// LuckFox TX -> D0 (RX0)
// LuckFox RX -> D1 (TX0)
// GND in common. Pro Mini and LuckFox are both 3.3V.
//
// Node i (1-based), Pro Mini TX -> node RX, Pro Mini RX -> node TX:
//   1: RX D2  TX D3     reset A0
//   2: RX D4  TX D5     reset A1
//   3: RX D6  TX D7     reset A2
//   4: RX D8  TX D9     reset A3
//   5: RX D10 TX D11    reset A4
//   6: RX D12 TX D13    reset A5   (D13 also drives the onboard LED)
//
// Reset pins are active-low and released (input pullup) when idle.
//
// Lines from the LuckFox, '\n' terminated:
//   !PING          -> OK
//   !SEL <n>       -> OK | ERR range
//   !RST <n>       -> OK | ERR range     pulse reset, does not need the node OS
//   !CONSOLE <n>   -> OK, then a raw bridge until a line !EXIT
//   anything else  -> forwarded to the selected node; one reply line comes back
//                      or ERR noselect / ERR timeout
//
// Node replies are streamed, not buffered: a Pro Mini has 2 KB RAM.

#define NUM_NODES 4

#if NUM_NODES < 1 || NUM_NODES > 6
#error NUM_NODES must be 1..6
#endif

#define _SS_MAX_RX_BUFF 32
#include <SoftwareSerial.h>

static const uint32_t HUB_BAUD = 115200;
static const uint32_t NODE_BAUD = 9600;
static const uint16_t RESET_MS = 120;
static const uint16_t FIRST_BYTE_MS = 500;
static const uint16_t LINE_MS = 4000;

static const uint8_t RESET_PINS[6] = {A0, A1, A2, A3, A4, A5};

SoftwareSerial node0(2, 3);
SoftwareSerial node1(4, 5);
SoftwareSerial node2(6, 7);
SoftwareSerial node3(8, 9);
#if NUM_NODES > 4
SoftwareSerial node4(10, 11);
SoftwareSerial node5(12, 13);
#endif

static int8_t selected = -1;

static SoftwareSerial* portOf(uint8_t index) {
  switch (index) {
    case 0: return &node0;
    case 1: return &node1;
    case 2: return &node2;
    case 3: return &node3;
#if NUM_NODES > 4
    case 4: return &node4;
    case 5: return &node5;
#endif
    default: return nullptr;
  }
}

static void flushPort(SoftwareSerial* port) {
  while (port->available() > 0) {
    port->read();
  }
}

static bool selectNode(uint8_t id) {
  if (id < 1 || id > NUM_NODES) {
    return false;
  }
  SoftwareSerial* port = portOf(id - 1);
  port->listen();
  delay(2);
  flushPort(port);
  selected = (int8_t)(id - 1);
  return true;
}

static void pulseReset(uint8_t id) {
  uint8_t pin = RESET_PINS[id - 1];
  pinMode(pin, OUTPUT);
  digitalWrite(pin, LOW);
  delay(RESET_MS);
  pinMode(pin, INPUT_PULLUP);
}

// Relay one node line to the LuckFox. Returns false on timeout.
static bool relayLine(SoftwareSerial* port) {
  bool saw = false;
  uint32_t start = millis();
  uint32_t last = start;
  while ((uint16_t)(millis() - start) < LINE_MS) {
    if (port->available() > 0) {
      char c = (char)port->read();
      if (c == '\r') {
        continue;
      }
      Serial.write(c);
      saw = true;
      last = millis();
      if (c == '\n') {
        return true;
      }
      continue;
    }
    if (!saw && (uint16_t)(millis() - start) >= FIRST_BYTE_MS) {
      return false;
    }
    if (saw && (uint16_t)(millis() - last) >= 80) {
      Serial.write('\n');
      return true;
    }
  }
  if (saw) {
    Serial.write('\n');
  }
  return saw;
}

static void forward(const char* cmd) {
  if (selected < 0) {
    Serial.println(F("ERR noselect"));
    return;
  }
  SoftwareSerial* port = portOf((uint8_t)selected);
  port->listen();
  port->print(cmd);
  port->write('\n');
  if (!relayLine(port)) {
    Serial.println(F("ERR timeout"));
  }
}

// Blocks until '\n'. Returns false if the command did not fit.
static bool readCommand(char* buf, uint8_t cap) {
  uint8_t n = 0;
  for (;;) {
    while (Serial.available() <= 0) {
    }
    char c = (char)Serial.read();
    if (c == '\r') {
      continue;
    }
    if (c == '\n') {
      buf[n] = 0;
      return true;
    }
    if ((uint8_t)(n + 1) >= cap) {
      for (;;) {
        while (Serial.available() <= 0) {
        }
        if (Serial.read() == '\n') {
          break;
        }
      }
      buf[0] = 0;
      return false;
    }
    buf[n++] = c;
  }
}

static bool validId(int id) {
  return id >= 1 && id <= NUM_NODES;
}

static int commandId(const char* cmd, const char* name) {
  size_t n = strlen(name);
  if (strncmp(cmd, name, n) != 0) {
    return -2;
  }
  if (cmd[n] == 0) {
    return -1;
  }
  if (cmd[n] != ' ') {
    return -2;
  }
  return atoi(cmd + n + 1);
}

static const char EXIT_LINE[] = "!EXIT\n";

static void consoleBridge(SoftwareSerial* port) {
  uint8_t match = 0;
  for (;;) {
    if (Serial.available() > 0) {
      char c = (char)Serial.read();
      if (c == EXIT_LINE[match]) {
        match++;
        if (EXIT_LINE[match] == 0) {
          Serial.println(F("OK"));
          return;
        }
      } else {
        for (uint8_t i = 0; i < match; i++) {
          port->write(EXIT_LINE[i]);
        }
        match = 0;
        if (c == EXIT_LINE[0]) {
          match = 1;
        } else {
          port->write(c);
        }
      }
    }
    if (port->available() > 0) {
      Serial.write((char)port->read());
    }
  }
}

static void handle(char* cmd) {
  if (cmd[0] == 0) {
    return;
  }

  if (strcmp(cmd, "!PING") == 0) {
    Serial.println(F("OK"));
    return;
  }

  int id = commandId(cmd, "!SEL");
  if (id != -2) {
    if (!validId(id) || !selectNode((uint8_t)id)) {
      Serial.println(F("ERR range"));
      return;
    }
    Serial.println(F("OK"));
    return;
  }

  id = commandId(cmd, "!RST");
  if (id != -2) {
    if (!validId(id)) {
      Serial.println(F("ERR range"));
      return;
    }
    pulseReset((uint8_t)id);
    Serial.println(F("OK"));
    return;
  }

  id = commandId(cmd, "!CONSOLE");
  if (id != -2) {
    if (id == -1) {
      if (selected < 0) {
        Serial.println(F("ERR noselect"));
        return;
      }
    } else if (!validId(id) || !selectNode((uint8_t)id)) {
      Serial.println(F("ERR range"));
      return;
    }
    Serial.println(F("OK"));
    consoleBridge(portOf((uint8_t)selected));
    return;
  }

  if (cmd[0] == '!') {
    Serial.println(F("ERR unknown"));
    return;
  }
  forward(cmd);
}

void setup() {
  Serial.begin(HUB_BAUD);
  node0.begin(NODE_BAUD);
  node1.begin(NODE_BAUD);
  node2.begin(NODE_BAUD);
  node3.begin(NODE_BAUD);
#if NUM_NODES > 4
  node4.begin(NODE_BAUD);
  node5.begin(NODE_BAUD);
#endif
  for (uint8_t i = 0; i < NUM_NODES; i++) {
    pinMode(RESET_PINS[i], INPUT_PULLUP);
  }
}

void loop() {
  char cmd[40];
  if (!readCommand(cmd, sizeof cmd)) {
    Serial.println(F("ERR long"));
    return;
  }
  handle(cmd);
}
