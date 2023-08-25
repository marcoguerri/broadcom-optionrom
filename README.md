`broadcom-optionrom` is a tool built based on the [reverse engineering](https://marcoguerri.github.io/reversing/msdos/2023/02/04/broadcom-pxe-write.html) 
of Boardcom MS-DOS utility B57UDIAG.EXE, specifically the control paths which manipulate OptionROM in NVRAM. It can be used to write
custom UEFI OptionROM to NVRAM of Broadcom BCM5751 NICs. It will likely work also for families of similar devices (e.g. BCM5718), but
so far it has been tested only on a BCM5751-RJ45 PCIe x1 1G Single-port Desktop Adapter.

# Usage
The tool supports two commands:

* `check`: performs NVRAM checks for UEFI OptionROM (e.g. directory checksum, OptionROM checksum, VPD checksum)
* `write`: write custom OptionROM on a binary dump of NIC NVRAM

# Example

First, the current content of NIC NRAM must be dumped via `ethtool`:

```
# ethtool -e <INTERFACE> raw on > NVRAM.bin
```

We can verify that all checks implemented by the tool do succeed:

```
$ go run main.go check -inNvramBinPath NVRAM.bin    
OK
```

`write` command can then be used to overwrite UEFI OptionROM:

```
$ go run main.go write -uefiOptionRomPath optionrom.v2.efi -inNvramBinPath NVRAM.bin -outNvramBinPath NVRAM.mod.bin
OK
```

We can check that the newly generated NVRAM image passes integrity checks:

```
$ go run main.go check -inNvramBinPath NVRAM.mod.bin
OK
```

The new NVRAM binary file can be written back to the device:
```
# ethtool -E <INTERFACE> magic 0x669955aa offset 0 length <FILE_LEN> < NVRAM.mod.bin
```

# Testing of physical device
The system I used for testing consists of a [PCEngines APU2D4 board](https://www.pcengines.ch/apu2d2.htm). This is normally used to 
implement routers, firewalls, etc, but it’s a great setup to run firmware experiments, thanks to its support for Open System Firmware 
(coreboot), potentially with UEFI payload, and easy access to SPI flash with external programmers.

![APU2D4](https://github.com/marcoguerri/broadcom-optionrom/img/apu2d.jpg)

For OptionROM experiments, I build coreboot with UEFI pyload support (edk2-tianocore) through `UefiPayloadPkg/UefiPayloadPkg.dsc`. 
Support for enumerating PCIe bus and dispatching execution to UEFI OptionROMs was patched on top of upstream. Back then, [Patrick Rudolph's
patch had not been merged yet](https://github.com/tianocore/edk2/pull/2693), but I see it's now close, which is great news. Bus enumeration 
would normally be coreboot responsibility, but nothing prevents UEFI from scanning the bus again, identifying UEFI OptionROMs and executing 
them. 

# Limitations
The tool has the following limitations, some of which are [further analyzed in the corresponding post](https://marcoguerri.github.io/reversing/msdos/2023/02/04/broadcom-pxe-write.html)
* It doesn't rewrite VendorID nor DeviceID in the OptionROM header based on the device we are working on. These two fields must be populated
correctly at OptionROM build time.


# Corrupted NVRAM


