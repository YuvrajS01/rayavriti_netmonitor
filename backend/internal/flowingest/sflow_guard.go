package flowingest

import (
	"encoding/binary"
	"fmt"

	"github.com/netsampler/goflow2/v2/decoders/sflow"
)

// maxSFlowRecords mirrors goflow2's own per-sample record cap.
const maxSFlowRecords = 1000

// checkSFlowRecordCounts rejects expanded flow samples advertising more than
// maxSFlowRecords records. goflow2 (v2.2.7) caps the count for every sample
// format except expanded flow samples, where it allocates the advertised
// count up front: one spoofed datagram could otherwise request a
// multi-gigabyte allocation. Other structural errors are left to the decoder.
func checkSFlowRecordCounts(data []byte) error {
	be := binary.BigEndian
	if len(data) < 8 {
		return nil
	}
	off := 8 // version, agent IP version
	switch be.Uint32(data[4:8]) {
	case 1:
		off += 4
	case 2:
		off += 16
	default:
		return nil
	}
	off += 12 // sub-agent ID, sequence, uptime
	if len(data) < off+4 {
		return nil
	}
	n := be.Uint32(data[off:])
	off += 4
	for i := uint32(0); i < n && len(data) >= off+8; i++ {
		format, length := be.Uint32(data[off:]), be.Uint32(data[off+4:])
		off += 8
		if uint64(length) > uint64(len(data)-off) {
			return nil
		}
		// Expanded flow sample: the record count follows ten 32-bit fields.
		if format == sflow.SAMPLE_FORMAT_EXPANDED_FLOW && length >= 44 {
			if c := be.Uint32(data[off+40:]); c > maxSFlowRecords {
				return fmt.Errorf("expanded flow sample with %d records", c)
			}
		}
		off += int(length)
	}
	return nil
}
