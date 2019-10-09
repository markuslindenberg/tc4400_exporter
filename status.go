package main

type SystemStatus struct {
	AcquiredDownstreamChannelFrequency uint64
	AcquiredDownstreamChannelLocked    bool
	ConnectivityOperational            bool
	BootOperational                    bool
	SecurityEnabled                    bool
	ConfigurationFile                  string
}

type ChannelStatus struct {
	Index                  int
	ID                     int
	Locked                 bool
	Type                   string
	Bonded                 bool
	CenterFrequency        uint64
	Width                  uint64
	SNR                    float64
	Level                  float64
	Modulation             string
	UnerroredCodewords     uint64
	CorrectedCodewords     uint64
	UncorrectableCodewords uint64
}
