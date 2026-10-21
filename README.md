# infinitive
Infinitive impersonates a SAM on a Carrier Infinity system management bus. 

## **DISCLAIMER**
**The software and hardware described here interacts with a proprietary system using information gleaned by reverse engineering.  Although it works fine for me, no guarantee or warranty is provided.  Use at your own risk to your HVAC system and yourself.**

## Getting started
#### Hardware setup
I've done all my development on a Raspberry Pi, although any reasonably performant Linux system with an RS-485 interface should work.  I chose the Pi 3 since the built-in WiFi saved me the hassle of running Ethernet to my furnace.  I'm not sure if older Pis have enough horsepower to run the Infinitive software.  If you give it a shot and are successful, please let me know so I can update this information.

In addition to a Linux system, you'll need to source an adapter to communicate on the RS-485 bus.  I am using a FTDI chipset USB to RS-485 adapter that I bought from Amazon.  There are a variety of adapters on Amazon and eBay, although it may take a few attempts to find one that works reliably.

Once you have a RS-485 adapter you'll need to connect it to your ABCD bus. The easiest way to do this is by attaching new wires to the A and B terminals of the ABCD bus connector inside your furnace and connecting them to your adapter. The A and B lines are used for RS-485 communication, while C and D are 24V AC power. **Do not connect your RS-485 adapter to the C and D terminals unless you want to see its magic smoke.** 

#### Software
Download the Infinitive release appropriate for your architecture.

   * amd64:
```
$ wget -O infinitive https://github.com/acd/infinitive/releases/download/v0.2/infinitive.amd64
```
   * arm:
```
$ wget -O infinitive https://github.com/acd/infinitive/releases/download/v0.2/infinitive.arm
```

Start Infinitive, providing the HTTP port to listen on for the management interface and the path to the correct serial device.

```
$ ./infinitive -httpport=8080 -serial=/dev/ttyUSB0 
```

Logs are written to stderr.  For now I've been running Infinitive under screen.  If folks are interested in a proper start/stop script and log management, submit a pull request or let me know.

If the RS-485 adapter is properly connected to your ABCD bus you should immediately see Infinitive logging messages indicating it is receiving data, such as:

```
INFO[0000] read frame: 2001 -> 4001: READ     000302    
INFO[0000] read frame: 4001 -> 2001: RESPONSE 000302041100000414000004020000 
INFO[0000] read frame: 2001 -> 4001: READ     000316    
INFO[0000] read frame: 4001 -> 2001: RESPONSE 0003160000000003ba004a2f780100037a 
```

Browse to your host system's IP, with the port you provided on the command line, and you should see a page that looks similar to the following:

<img src="https://raw.githubusercontent.com/acd/infinitive/master/screenshot.png"/>

**Note:** I am not much of a frontend web developer.  I'd love to see pull requests for enhancements to the web interface.

There is a brief delay between altering a setting and Infinitive updating the information displayed.  This is due to Infinitive polling the thermostat settings once per second.

## DS18B20 temperature probes (Raspberry Pi)

Infinitive can read two DS18B20 1-Wire temperature probes connected to the Raspberry Pi's GPIO header and show them on the web page as **Wine (top)** and **Wine (bottom)**. Both probes share a single bus, so they use one GPIO pin.

#### Parts
* 2 x DS18B20 probes (the waterproof 3-wire type, or bare TO-92 sensors)
* 1 x 4.7 kΩ resistor (a 10 kΩ will often work, but 4.7 kΩ is the standard value)
* Jumper wires or a small terminal block / breadboard

#### Wiring
The Pi 4 header pins used are **pin 1 (3.3V)**, **pin 7 (GPIO4)** and **pin 9 (GND)**. Pin numbers are physical header positions. Pin 1 is the corner pin at the end of the header nearest the SD card slot, and it is the one with the square solder pad on the underside of the board. Odd-numbered pins (1, 3, 5, 7, 9 ...) are in the row farther from the board edge, so pin 1, pin 7 and pin 9 are all in the same row: counting from the SD card end, pin 7 is the 4th pin and pin 9 is the 5th.

| DS18B20 wire (typical colors) | Connects to                         |
|-------------------------------|-------------------------------------|
| Red (VDD)                     | Pi pin 1 (3.3V)                     |
| Black (GND)                   | Pi pin 9 (GND)                      |
| Yellow / white (DATA)         | Pi pin 7 (GPIO4)                    |

```
                      3.3V (pin 1) o------+---------------+------------------+
                                          |               |                  |
                                       [ 4.7k ]       Probe 1 VDD        Probe 2 VDD
                                          |           (red)              (red)
 GPIO4 / 1-Wire (pin 7) o-----------------+---------- Probe 1 DATA ------ Probe 2 DATA
                                                      (yellow)            (yellow)
                      GND (pin 9) o------------------- Probe 1 GND ------- Probe 2 GND
                                                      (black)             (black)
```

1. Connect **both probes in parallel**: all red wires together to 3.3V, all black wires together to GND, and all yellow (data) wires together to GPIO4.
2. Place the **4.7 kΩ pull-up resistor between the DATA line and 3.3V**. You need only one resistor for the whole bus, not one per probe.
3. Always power the probes from **3.3V, not 5V**. 5V on the data line can damage the Pi's GPIO.
4. Wire colors vary between manufacturers. Many waterproof probes are red = VDD, black = GND, and yellow *or* white = DATA, but check your probe's documentation before powering up. A probe wired backwards (VDD and GND swapped) gets hot very quickly and is destroyed.
5. Keep the wiring short and tidy. Runs of several meters work fine with a 4.7 kΩ resistor, but very long runs may need a smaller pull-up (for example 2.2 kΩ) or twisted-pair cable.

Power the Pi off before connecting or changing wiring.

#### Enable the 1-Wire interface
Add the 1-Wire overlay to `/boot/firmware/config.txt` (`/boot/config.txt` on older Raspberry Pi OS releases):

```
dtoverlay=w1-gpio,gpiopin=4
```

Reboot, then check that both probes are detected:

```
$ ls /sys/bus/w1/devices/
28-0316a2795e3b  28-0416b3a1c2d4  w1_bus_master1
$ cat /sys/bus/w1/devices/28-0316a2795e3b/w1_slave
72 01 4b 46 7f ff 0e 10 57 : crc=57 YES
72 01 4b 46 7f ff 0e 10 57 t=23125
```

Each probe shows up as a directory named `28-` followed by its unique serial number. `t=23125` is the temperature in thousandths of a degree Celsius (23.125 °C). If you see no `28-...` directories, re-check the wiring and the pull-up resistor. A reading of exactly 85 °C (`t=85000`) means the probe powered up but did not complete a conversion, which almost always points to a wiring or pull-up problem.

#### Telling the probes apart
Run Infinitive with the probes enabled and it will assign the two sensors it finds, sorted by ID, to **Wine (top)** and **Wine (bottom)** in that order. That default is stable but arbitrary, so identify which physical probe is which:

1. Start Infinitive and note the temperatures shown on the page.
2. Warm one probe in your hand, or hold it in a glass of cool water, and see which reading moves. Then look up its ID with `cat /sys/bus/w1/devices/28-*/w1_slave` while doing the same.
3. Pin the IDs with flags so the labels can never silently swap if a probe is replaced:

```
$ ./infinitive -httpport=8080 -serial=/dev/ttyUSB0 \
      -probe-top=28-0316a2795e3b -probe-bottom=28-0416b3a1c2d4
```

Pass `-probes=false` to disable probe support entirely. If the 1-Wire bus is not present (for example when running on a non-Pi machine), the page shows `--` with an explanatory message instead of a temperature.

Probes are read every 5 seconds. Each reading takes about three quarters of a second, and the two probes are read concurrently.

## Building from source

If you'd like to build Infinitive from source, first confirm you have a working Go environment (I've been using release 1.21).  Ensure your GOPATH and GOHOME are set correctly, then:

```
$ go get github.com/acd/infinitive
$ go build github.com/acd/infinitive
```

## JSON API

Infinitive exposes a JSON API to retrieve and manipulate thermostat parameters.

#### GET /api/zone/1/config

```json
{
   "currentTemp": 70,
   "currentHumidity": 50,
   "outdoorTemp": 50,
   "mode": "heat",
   "stage": 2,
   "fanMode": "auto",
   "hold": true,
   "heatSetpoint": 68,
   "coolSetpoint": 74,
   "rawMode": 64
}
```
rawMode included for debugging purposes. It encodes stage and mode. 

#### PUT /api/zone/1/config

Valid values for zone are 1-8.

```json
{
   "mode": "auto",
   "fanMode": "auto",
   "hold": true,
   "heatSetpoint": 68,
   "coolSetpoint": 74
}
```

Valid write values for `mode` are `off`, `auto`, `heat`, and `cool`.
Additional read values for mode are `electric` and `heatpump` indicating "heat pump only" or "electric heat only" have been selected at the thermostat 
Values for `fanMode` are `auto`, `low`, `med`, and `high`.

#### GET /api/airhandler

```json
{
	"blowerRPM":0,
	"airFlowCFM":0,
	"elecHeat":false
}
```

#### GET /api/heatpump

```json
{
	"coilTemp":28.8125,
	"outsideTemp":31.375,
	"stage":2
}
```


#### GET /api/probes

Returns the DS18B20 probe readings (see [DS18B20 temperature probes](#ds18b20-temperature-probes-raspberry-pi)). Temperatures are provided in both Celsius and Fahrenheit. If a probe cannot be read, `ok` is `false`, the temperatures are `null`, and `error` explains why.

```json
{
   "top":    {"label":"Wine (top)",    "id":"28-0316a2795e3b", "ok":true, "tempC":13.1, "tempF":55.6},
   "bottom": {"label":"Wine (bottom)", "id":"28-0416b3a1c2d4", "ok":true, "tempC":12.8, "tempF":55.0}
}
```

The same data is pushed over the `/api/ws` websocket with source `probes`.

#### GET /api/zone/1/vacation

```
{
   "active":false,
   "days":0,
   "minTemperature":56,
   "maxTemperature":84,
   "minHumidity":15,
   "maxHumidity":60,
   "fanMode":"auto"
}
```

#### PUT /api/zone/1/vacation

```
{
   "days":0,
   "minTemperature":56,
   "maxTemperature":84,
   "minHumidity":15,
   "maxHumidity":60,
   "fanMode":"auto"
}
```

All parameters are optional.  A single parameter may be updated by sending a JSON document containing only that parameter.  Vacation mode is disabled by setting `days` to `0`.  Valid values for `fanMode` are `auto`, `low`, `med`, and `high`.

## Details
#### ABCD bus
Infinity systems use a proprietary binary protocol for data exchange between system components.  These message are sent across an RS-485 serial bus which Carrier refers to as the ABCD bus.  Most systems usually includes an air-conditioning unit or heat pump, furnace, and thermostat.  The thermostat is responsible for enumerating other components of the system and managing their operation. 

The protocol has been reverse engineered as Carrier has not published a protocol specification.  The following resources provided invaluable assistance with my reverse engineering efforts:

* [Cocoontech's Carrier Infinity Thread](http://cocoontech.com/forums/topic/11372-carrier-infinity/)
* [Infinitude Wiki](https://github.com/nebulous/infinitude/wiki/Infinity-Protocol-Main)

Infinitive reads and writes information from the Infinity thermostat.  It also gathers data by passively observing traffic exchanged between the thermostat and other system components.

#### Bryant Evolution
I believe Infinitive should work with Bryant Evolution systems as they use the same ABCD bus.  Please let me know if you have success using Infinitive on a Bryant system.

#### Unimplemented features

Multi-zone Infinity HVAC systems are partially supported.  For now, the set API supports manipulating zonal fan mode, heat setpoint, and cool setpoint.  I only have a single zone setup, so I can't test if multi-zone capability works properly.  If you have a multi-zone setup and want to be a guinea pig, get in touch and maybe we can work something out.

I don't use the thermostat's scheduling capabilities or vacation mode so Infinitive does not support them.  Reach out if this is something you'd like to see.  

#### Issues
##### rPi USB stack
The USB to RS-485 adapter I'm using periodically locks up due to what appear to be USB stack issues on the Raspberry Pi 3.  When this happens, reads on the serial file descriptor block forever and the kernel logs the following:
```
[491862.396039] ftdi_sio ttyUSB0: usb_serial_generic_read_bulk_callback - urb stopped: -32
```
Infinitive reopens the serial interface when it hasn't received any data in 5 seconds to workaround the issue.  Alternatively, forcing the Pi USB stack to USB 1.1 mode resolves the issue.  If you want to go this route, add `dwc_otg.speed=1` to `/boot/config.txt` and reboot the Pi.

##### Bogus data
Occasionally Infinitive will display incorrect data via the web interface for a second.  This is likely caused by improper parsing of data received from the ABCD bus.  I'd like to track down the root cause of this issue and resolve it, but due to its transient nature it's not a high priority and does not affect usability.

#### See Also
[Infinitude](https://github.com/nebulous/infinitude) is another solution for managing Carrier HVAC systems.  It impersonates Carrier web services and provides an alternate interface for controlling Carrier Internet-enabled touchscreen thermostats.  It also supports passive snooping of the RS-485 bus and can decode and display some of the data.

#### Contact
Andrew Danforth (<adanforth@gmail.com>)
