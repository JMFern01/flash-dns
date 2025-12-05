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

type QueryInfo struct {
	Domain   string
	CacheKey string
	QType    uint16
	QClass   uint16
}

func ParseQuery(query []byte) (*QueryInfo, error) {
	var queryLength int = len(query)
	if queryLength < 12 {
		return nil, fmt.Errorf("query too short: %d bytes", len(query))
	}

	var (
		builder  *strings.Builder = builderPool.Get().(*strings.Builder)
		position int              = 12
		length   int              = 0
		domain   string
		qtype    uint16
		qclass   uint16
		cacheKey string
	)
	builder.Reset()
	defer builderPool.Put(builder)

	for position < queryLength {
		length = int(query[position])

		if length == 0 {
			position++
			break
		}

		if length >= 192 {
			position += 2
			continue
		}

		if builder.Len() > 0 {
			builder.WriteRune('.')
		}
		position++

		if position+length > queryLength {
			return nil, fmt.Errorf("invalid domain name length")
		}

		builder.Write(query[position : position+length])
		position += length
	}
	domain = builder.String()

	if position+4 > queryLength {
		return nil, fmt.Errorf("query too short for QTYPE/QCLASS")
	}

	qtype = binary.BigEndian.Uint16(query[position : position+2])
	qclass = binary.BigEndian.Uint16(query[position+2 : position+4])

	cacheKey = fmt.Sprintf("%s:%d", domain, qtype)

	return &QueryInfo{Domain: domain, QType: qtype, QClass: qclass, CacheKey: cacheKey}, nil
}

func ExtractTTL(response []byte) uint32 {
	if len(response) < 12 {
		return 300 // 5 minutes -> 60 * 5 = 300
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
