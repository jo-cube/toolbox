package kshape

import (
	"bufio"
	"fmt"
	"io"
	"math"

	"github.com/jo-cube/toolbox/internal/prob"
)

const CanonicalFormat = `%t\t%p\t%o\t%T\t%S\t%K\t%k\n`

type Record struct {
	Topic         string
	Partition     int32
	Offset        int64
	Timestamp     int64
	PayloadLength int64
	KeyHash       uint64
	NullKey       bool
}

func Build(r io.Reader, bucketWidth uint64, precision uint8) (*Summary, error) {
	s, err := New(bucketWidth, precision)
	if err != nil {
		return nil, err
	}
	if err := eachRecord(r, s.Add); err != nil {
		return nil, err
	}
	return s, nil
}

func eachRecord(r io.Reader, fn func(Record) error) error {
	br := bufio.NewReaderSize(r, 64<<10)
	var scratch [32 << 10]byte
	var topic string

	for number := uint64(1); ; number++ {
		rawTopic, err := br.ReadSlice('\t')
		if err == io.EOF && len(rawTopic) == 0 {
			return nil
		}
		if err != nil {
			return fmt.Errorf("record %d: read topic: %w", number, err)
		}
		rawTopic = rawTopic[:len(rawTopic)-1]
		if len(rawTopic) == 0 || len(rawTopic) > 249 {
			return fmt.Errorf("record %d: invalid topic length %d", number, len(rawTopic))
		}
		if topic == "" {
			topic = string(rawTopic)
		} else if !equalStringBytes(topic, rawTopic) {
			return fmt.Errorf("record %d: topic %q differs from %q", number, rawTopic, topic)
		}

		partition, err := readDecimalField(br, "partition")
		if err != nil {
			return fmt.Errorf("record %d: %w", number, err)
		}
		if partition < 0 || partition > math.MaxInt32 {
			return fmt.Errorf("record %d: partition %d is out of range", number, partition)
		}
		offset, err := readDecimalField(br, "offset")
		if err != nil {
			return fmt.Errorf("record %d: %w", number, err)
		}
		if offset < 0 {
			return fmt.Errorf("record %d: offset must be non-negative", number)
		}
		timestamp, err := readDecimalField(br, "timestamp")
		if err != nil {
			return fmt.Errorf("record %d: %w", number, err)
		}
		payloadLength, err := readDecimalField(br, "payload length")
		if err != nil {
			return fmt.Errorf("record %d: %w", number, err)
		}
		if payloadLength < -1 {
			return fmt.Errorf("record %d: payload length must be -1 or non-negative", number)
		}
		keyLength, err := readDecimalField(br, "key length")
		if err != nil {
			return fmt.Errorf("record %d: %w", number, err)
		}
		if keyLength < -1 {
			return fmt.Errorf("record %d: key length must be -1 or non-negative", number)
		}

		record := Record{
			Topic:         topic,
			Partition:     int32(partition),
			Offset:        offset,
			Timestamp:     timestamp,
			PayloadLength: payloadLength,
			NullKey:       keyLength == -1,
		}
		if keyLength >= 0 {
			h := prob.NewHasher64(0)
			for remaining := keyLength; remaining > 0; {
				n := min(remaining, int64(len(scratch)))
				if _, err := io.ReadFull(br, scratch[:n]); err != nil {
					return fmt.Errorf("record %d: read %d-byte key: %w", number, keyLength, err)
				}
				h.Write(scratch[:n])
				remaining -= n
			}
			record.KeyHash = h.Sum64()
		}
		terminator, err := br.ReadByte()
		if err != nil {
			return fmt.Errorf("record %d: read record terminator: %w", number, err)
		}
		if terminator != '\n' {
			return fmt.Errorf("record %d: invalid record terminator 0x%02x", number, terminator)
		}
		if err := fn(record); err != nil {
			return fmt.Errorf("record %d: %w", number, err)
		}
	}
}

func equalStringBytes(value string, data []byte) bool {
	if len(value) != len(data) {
		return false
	}
	for i := range data {
		if value[i] != data[i] {
			return false
		}
	}
	return true
}

func readDecimalField(br *bufio.Reader, name string) (int64, error) {
	field, err := br.ReadSlice('\t')
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", name, err)
	}
	value, err := parseDecimal(field[:len(field)-1])
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, field[:len(field)-1], err)
	}
	return value, nil
}

func parseDecimal(value []byte) (int64, error) {
	if len(value) == 0 {
		return 0, fmt.Errorf("empty integer")
	}
	negative := value[0] == '-'
	if negative {
		value = value[1:]
		if len(value) == 0 {
			return 0, fmt.Errorf("empty integer")
		}
	}
	limit := uint64(math.MaxInt64)
	if negative {
		limit++
	}
	var result uint64
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return 0, fmt.Errorf("not a decimal integer")
		}
		n := uint64(digit - '0')
		if result > (limit-n)/10 {
			return 0, fmt.Errorf("integer overflow")
		}
		result = result*10 + n
	}
	if negative {
		if result == uint64(math.MaxInt64)+1 {
			return math.MinInt64, nil
		}
		return -int64(result), nil
	}
	return int64(result), nil
}
