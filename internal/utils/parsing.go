package utils

import (
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
)

var builderPool = sync.Pool{
	New: func() interface{} {
		return &strings.Builder{}
	},
}

func parseDomainName(data []byte, offset int) (string, int) {
	var builder *strings.Builder = builderPool.Get().(*strings.Builder)
	builder.Reset()
	defer builderPool.Put(builder)

	var (
		position int = offset
		length   int
	)
	for {
		length = int(data[position])
		if length == 0 { // if zero, then it is the null terminator
			position++
			break
		}

		if builder.Len() > 0 {
			builder.WriteRune('.')
		}

		position++ // skips the length byte
		builder.Write(data[position : position+length])
		position += length // goes to the next length byte
	}

	return builder.String(), position
}

func ExtractTTL(response []byte) uint32 {
	var result uint32 = 300 //default to 5 minutes (300 seconds)

	if len(response) < 12 {
		return result
	}

	// skip header (12 bytes)
	var position int = 12

	// skip question section
	var qdcount uint16 = binary.BigEndian.Uint16(response[4:6])
	for i := 0; i < int(qdcount); i++ {

		//skip domain name
		for position < len(response) && response[position] != 0 {
			if response[position] >= 192 { //compression pointer
				position += 2
				break
			}
			position += int(response[position]) + 1
		}

		if response[position] == 0 {
			position++
		}

		position += 4 // skip QTYPE and QCLASS
	}

	// read answer section to find TTL
	var ancount uint16 = binary.BigEndian.Uint16(response[6:8])
	var minTTL uint32 = uint32(3600) // default 1 hour

	for i := 0; i < int(ancount) && position+10 < len(response); i++ {
		//skip name
		for position < len(response) && response[position] != 0 {
			if response[position] >= 192 { // compression pointer
				position += 2
				break
			}

			position += int(response[position]) + 1
		}

		if position < len(response) && response[position] == 0 {
			position++
		}

		if position+10 > len(response) {
			break
		}

		var ttl uint32 = binary.BigEndian.Uint32(response[position+4 : position+8])
		if ttl < minTTL {
			minTTL = ttl
		}

		// skip type, class, ttl and rdlength
		var rdlength uint16 = binary.BigEndian.Uint16(response[position+8 : position+10])
		position += 10 + int(rdlength)
	}

	return minTTL
}

func CreateCacheKey(query []byte) string {
	var (
		domain           string
		position         int
		qtype            uint16
		skipHeaderOffset int = 12
	)
	if len(query) < 12 {
		return ""
	}

	domain, position = parseDomainName(query, skipHeaderOffset)
	if position > len(query) {
		return domain
	}

	qtype = binary.BigEndian.Uint16(query[position : position+2])
	return fmt.Sprintf("%s:%d", domain, qtype)
}
