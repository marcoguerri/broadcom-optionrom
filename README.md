`broadcom-optionrom` is a tool built based on the [reverse engineering](https://marcoguerri.github.io/reversing/msdos/2023/02/04/broadcom-pxe-write.html) 
of Boardcom MS-DOS utility `B57UDIAG.EXE`, specifically the control paths which manipulate OptionROM in NVRAM. It can be used to write
custom UEFI OptionROM to NVRAM of Broadcom BCM5751 NICs. It will likely work also for families of similar devices (e.g. BCM5718), but
so far it has been tested only on a BCM5751-RJ45 PCIe x1 1G Single-port Desktop Adapter.

# Usage
The tool supports two commands:

* `check`: performs NVRAM checks for UEFI OptionROM (e.g. directory checksum, OptionROM checksum, VPD checksum)
* `write`: writes custom OptionROM on a binary dump of NIC NVRAM

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

# Testing on physical device
The system I used for testing consists of a [PCEngines APU2D4 board](https://www.pcengines.ch/apu2d2.htm). This is normally used to 
implement routers, firewalls, etc, but it’s a great setup to run firmware experiments, thanks to its support for Open System Firmware 
(coreboot), potentially with UEFI payload, and easy access to SPI flash with external programmers.

![APU2D4](https://github.com/marcoguerri/broadcom-optionrom/blob/master/img/apu2d.jpg)

For OptionROM experiments, I build coreboot with UEFI pyload support (edk2-tianocore) through `UefiPayloadPkg/UefiPayloadPkg.dsc`. 
Support for enumerating PCIe bus and dispatching execution to UEFI OptionROMs was patched on top of upstream. Back then, [Patrick Rudolph's
patch had not been merged yet](https://github.com/tianocore/edk2/pull/2693), but I see it's now close, which is great news. Bus enumeration 
would normally be coreboot responsibility, but nothing prevents UEFI from scanning the bus again, identifying UEFI OptionROMs and executing 
them. 

An example of full end to end test would be the following:
 Current OptionROM is executing correctly:
```
InstallProtocolInterface: 4006C0C1-FCB3-403E-996D-4A6C8724E06D CE8414F0
[Security] 3rd party image[0] can be loaded after EndOfDxe: PciRoot(0x0)/Pci(0x2,0x1)/Pci(0x0,0x0)/Offset(0x0,0x1FFF).
InstallProtocolInterface: 5B1B31A1-9562-11D2-8E3F-00A0C969723B CE842C40
Loading driver at 0x000CE7D3000 EntryPoint=0x000CE7D3F92 OptionROM.efi
InstallProtocolInterface: BC62157E-3E33-4FEC-9920-2D3B36D750DF CE841C98
ProtectUefiImageCommon - 0xCE842C40
  - 0x00000000CE7D3000 - 0x0000000000001E40
*** Nel mezzo del cammin di nostra vita
*** mi ritrovai per una selva oscura
*** che la diritta via era smarrita
```

We built a new OptionROM. From:
```
    DEBUG((DEBUG_INFO, "*** Nel mezzo del cammin di nostra vita\n"));
    DEBUG((DEBUG_INFO, "*** mi ritrovai per una selva oscura\n"));
    DEBUG((DEBUG_INFO, "*** che la diritta via era smarrita\n"));
    return EFI_SUCCESS;
```
to:
```
    DEBUG((DEBUG_INFO, "*** This is a new OptionROM binary \n"));
    return EFI_SUCCESS;
```

Through EDK tooling, we assemble the new UEFI OptionROM binary, with the device specific VendorID and DeviceID:

```
$ ./BaseTools/Source/C/bin/EfiRom -f 0x1677 -i 0x14e4 -e Build/OvmfX64/DEBUG_GCC5/X64/OptionROM/OptionROM/DEBUG/OptionROM.efi -o optionrom.efi
```

We dump the current NVRAM:
```
# ethtool -e enp1s0 raw on > NVRAM.bin
```

and overwrite OptionROM:

```
$ go run main.go write -uefiOptionRomPath optionrom.efi -inNvramBinPath NVRAM.bin -outNvramBinPath NVRAM.mod.bin
$ OK
$ go run main.go check -inNvramBinPath NVRAM.mod.bin
OK
```

We then write the NVRAM back to the device:
```
# ethtool -E enp1s0 magic 0x669955aa offset 0 length 131072 < NVRAM.mod.bin
```
and reboot. We see the OptionROM changes reflected in its execution at UEFI boot time:
```
InstallProtocolInterface: 4006C0C1-FCB3-403E-996D-4A6C8724E06D CE8414F0
[Security] 3rd party image[0] can be loaded after EndOfDxe: PciRoot(0x0)/Pci(0x2,0x1)/Pci(0x0,0x0)/Offset(0x0,0x1DFF).
InstallProtocolInterface: 5B1B31A1-9562-11D2-8E3F-00A0C969723B CE842C40
Loading driver at 0x000CE7D5000 EntryPoint=0x000CE7D5F92 OptionROM.efi
InstallProtocolInterface: BC62157E-3E33-4FEC-9920-2D3B36D750DF CE841C98
ProtectUefiImageCommon - 0xCE842C40
  - 0x00000000CE7D5000 - 0x0000000000001DC0
*** This is a new OptionROM binary 
```

  
# Limitations
The tool has the following limitations, some of which are [further analyzed in the corresponding post](https://marcoguerri.github.io/reversing/msdos/2023/02/04/broadcom-pxe-write.html):
* It doesn't implement any type of NVRAM space look-up. It just searches for PXE entry in the directory and stores the new OptionROM starting
from the same offset. It really doesn't care if in the process, anything else might be overwritten. This is obvioulsy super naive, and 
implementing a proper [space look-up algorithm as described in the post](https://marcoguerri.github.io/reversing/msdos/2023/02/04/broadcom-pxe-write.html), would not be difficult. I have gotten around to implementing this yet.
* It doesn't rewrite VendorID nor DeviceID in the OptionROM header based on the device we are working on. These two fields must be populated
correctly at OptionROM build time.


# Corrupted NVRAM


