package go2dpf

import (
	"github.com/deadsy/libusb"
	"fmt"
	"log"
)

const (
	ax206vid = 0x1908 // Hacked frames USB Vendor ID
	ax206pid = 0x0102 // Hacked frames USB Product ID

	usbCmdSetProperty = 0x01 // USB command: Set property
	usbCmdBlit = 0x12 // USB command: Blit to screen

	ax206interface = 0x00
	ax206endpOut = 0x01
	ax206endpIn = 0x81
)

type DPF struct {
	Width  int
	Height int
	Debug  bool

	ctx      libusb.Context
	udev     libusb.Device_Handle
	hasCtx   bool
	hasUdev  bool
	hasClaim bool
}

func OpenDpf() (*DPF, error) {
	vid := uint16(ax206vid)
	pid := uint16(ax206pid)
	dpf := new(DPF)

	err := libusb.Init(&dpf.ctx)
	if err != nil {
		return nil, err
	}
	dpf.hasCtx = true

	dpf.udev = libusb.Open_Device_With_VID_PID(dpf.ctx, vid, pid)
	if dpf.udev == nil {
		return nil, fmt.Errorf("Failed to open device [%04x:%04x] (may be no permission?)", vid, pid)
	}
	dpf.hasUdev = true

	libusb.Set_Auto_Detach_Kernel_Driver(dpf.udev, true)

	if err = libusb.Claim_Interface(dpf.udev, ax206interface); err != nil {
		return nil, fmt.Errorf("Can't claim interface: %v", err)
	}
	dpf.hasClaim = true

	return dpf, nil
}

func (dpf *DPF) Close() {
	if dpf.hasClaim {
		libusb.Release_Interface(dpf.udev, ax206interface)
	}
	if dpf.hasUdev {
		libusb.Close(dpf.udev)
	}
	if dpf.hasCtx {
		libusb.Exit(dpf.ctx)
		dpf.hasCtx = false
	}
}

func (dpf *DPF) GetDimensions() (width, height int, err error) {
	cmd := []byte{
		0xcd, 0, 0, 0,
		0, 2, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
	}
	data, err := dpf.scsiRead(cmd, 5)
	if err != nil {
		return 0, 0, err
	}
	width = int(data[0]) | int(data[1])<<8
	height = int(data[2]) | int(data[3])<<8
	return width, height, nil
}

func (dpf *DPF) Brightness(lvl int) error {
	if lvl < 0 {
		lvl = 0
	}
	if lvl > 7 {
		lvl = 7
	}

	cmd := []byte{
		0xcd, 0, 0, 0,
		0, 6, usbCmdSetProperty,
		1, 0, // PROPERTY_BRIGHTNESS
		byte(lvl), byte(lvl >> 8),
		0, 0, 0, 0, 0,
	}

	return dpf.scsiWrite(cmd, nil)
}

func (dpf *DPF) Blit(img *ImageRGB565) error {

	r := img.Rect
	cmd := []byte{
		0xcd, 0, 0, 0,
		0, 6, usbCmdBlit,
		byte(r.Min.X), byte(r.Min.X >> 8),
		byte(r.Min.Y), byte(r.Min.Y >> 8),
		byte(r.Max.X-1), byte((r.Max.X-1) >> 8),
		byte(r.Max.Y-1), byte((r.Max.Y-1) >> 8),
		0,
	}
	return dpf.scsiWrite(cmd, img.PixRect())
}

const scsiTimeout = 1000

func (dpf *DPF) scsiCmdPrepare(cmd []byte, blockLen int, out bool) []byte {
	var bmCBWFlags byte
	if out {
		bmCBWFlags = 0x00
	} else {
		bmCBWFlags = 0x80
	}
	buf := []byte{
		0x55, 0x53, 0x42, 0x43, // dCBWSignature
		0xde, 0xad, 0xbe, 0xef, // dCBWTag
		byte(blockLen), byte(blockLen >> 8), byte(blockLen >> 16), byte(blockLen >> 24), // dCBWLength (4 byte)
		bmCBWFlags,     // bmCBWFlags: 0x80: data in (dev to host), 0x00: Data out
		0x00,           // bCBWLUN
		byte(len(cmd)), // bCBWCBLength

		// SCSI cmd: (15)
		0xcd, 0x00, 0x00, 0x00,
		0x00, 0x06, 0x11, 0xf8,
		0x70, 0x00, 0x40, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}

	copy(buf[15:], cmd)

	if dpf.Debug {
		log.Print("SCSI cmd: ", cmd)
		log.Print("SCSI command: ", buf)
	}
	return buf
}

func (dpf *DPF) scsiGetAck() error {
	buf := []byte{
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0,
	}
	// Get ACK
	if dpf.Debug {
		log.Print("[ACK] Read ACK from device")
	}
	ack, err := libusb.Bulk_Transfer(dpf.udev, ax206endpIn, buf, scsiTimeout)
	if err != nil {
		return err
	}
	if dpf.Debug {
		log.Print("[ACK] data ", ack)
	}

	if string(ack[:4]) != "USBS" {
		return fmt.Errorf("Got invalid reply")
	}
	// pass back return code set by peer:
	// return ansbuf[12];
	return nil
}

func (dpf *DPF) scsiWrite(cmd []byte, data []byte) error {
	var err error

	// Write command to device
	if dpf.Debug {
		log.Print("[WRITE] Write command to device")
	}
	_, err = libusb.Bulk_Transfer(dpf.udev, ax206endpOut, dpf.scsiCmdPrepare(cmd, len(data), true), scsiTimeout)
	if err != nil {
		return err
	}

	// Write data to device
	if data != nil {
		if dpf.Debug {
			log.Print("[WRITE] Write data to device")
		}
		_, err := libusb.Bulk_Transfer(dpf.udev, ax206endpOut, data, scsiTimeout)
		if err != nil {
			return err
		}
	}

	return dpf.scsiGetAck()
}

func (dpf *DPF) scsiRead(cmd []byte, blockLen int) ([]byte, error) {
	var err error

	// Write command to device
	if dpf.Debug {
		log.Print("[READ] Write command to device")
	}
	_, err = libusb.Bulk_Transfer(dpf.udev, ax206endpOut, dpf.scsiCmdPrepare(cmd, blockLen, false), scsiTimeout)
	if err != nil {
		return nil, err
	}

	if dpf.Debug {
		log.Print("[READ] Read data from device")
	}
	// Read data from device
	data1 := make([]byte, blockLen, blockLen)
	data, err := libusb.Bulk_Transfer(dpf.udev, ax206endpIn, data1, scsiTimeout)
	if err != nil {
		return nil, err
	}
	if dpf.Debug {
		log.Print("[READ] data ", data)
	}

	err = dpf.scsiGetAck()
	if err != nil {
		return data, err
	}

	return data, nil
}
