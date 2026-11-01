CC ?= gcc
ROOT := ..
CFLAGS ?= -Wall -Wextra -O2
CPPFLAGS += -I$(ROOT)/c_include
LDFLAGS += -L$(ROOT)/libs
LDFLAGS += -no-pie
SDK_LIB ?= dspreadsdk
LDLIBS += -l$(SDK_LIB)

ifeq ($(SDK_LIB),dspreadsdk-for-arm-arch)
CPPFLAGS += -DDSPREAD_ARM_CALLBACKS
endif

.PHONY: all arm clean

all: dspread_apdu_repro

arm:
	$(MAKE) -B SDK_LIB=dspreadsdk-for-arm-arch

dspread_apdu_repro: dspread_apdu_repro.c
	$(CC) $(CPPFLAGS) $(CFLAGS) $< -o $@ $(LDFLAGS) $(LDLIBS)

clean:
	rm -f dspread_apdu_repro
