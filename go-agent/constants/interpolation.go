package constants

// TinyML Vector Similarity Interpolation Constants (Single Source of Truth)
const (
	// DefaultDistanceThreshold is the default conservative geometric distance for nearest-neighbor matching
	DefaultDistanceThreshold = 2.5
	// MinDistanceThreshold is the lower sanity boundary for distance threshold configuration
	MinDistanceThreshold = 1.0
	// MaxDistanceThreshold is the upper sanity boundary for distance threshold configuration
	MaxDistanceThreshold = 5.0

	// DefaultDecayLambda is the default exponential decay rate for confidence score: Q = Q_neighbor * exp(-lambda * d)
	DefaultDecayLambda = 0.15
	// DecayLambda is an alias for DefaultDecayLambda for backward compatibility
	DecayLambda = 0.15
	// MinDecayLambda is the lower sanity boundary for confidence decay configuration
	MinDecayLambda = 0.01
	// MaxDecayLambda is the upper sanity boundary for confidence decay configuration
	MaxDecayLambda = 1.0
)
