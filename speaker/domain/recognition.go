package domain

// SpeakerIdentificationResult represents the result of a speaker identification operation
type SpeakerIdentificationResult struct {
	identified   bool
	speakerID    SpeakerID
	speakerName  SpeakerName
	confidence   SimilarityScore
	threshold    SimilarityScore
}

// NewSpeakerIdentificationResult creates a new identification result
func NewSpeakerIdentificationResult(
	identified bool,
	speakerID SpeakerID,
	speakerName SpeakerName,
	confidence SimilarityScore,
	threshold SimilarityScore,
) *SpeakerIdentificationResult {
	return &SpeakerIdentificationResult{
		identified:  identified,
		speakerID:   speakerID,
		speakerName: speakerName,
		confidence:  confidence,
		threshold:   threshold,
	}
}

// Identified returns whether a speaker was successfully identified
func (r *SpeakerIdentificationResult) Identified() bool {
	return r.identified
}

// SpeakerID returns the identified speaker's ID (empty if not identified)
func (r *SpeakerIdentificationResult) SpeakerID() SpeakerID {
	return r.speakerID
}

// SpeakerName returns the identified speaker's name (empty if not identified)
func (r *SpeakerIdentificationResult) SpeakerName() SpeakerName {
	return r.speakerName
}

// Confidence returns the similarity confidence score
func (r *SpeakerIdentificationResult) Confidence() SimilarityScore {
	return r.confidence
}

// Threshold returns the similarity threshold used
func (r *SpeakerIdentificationResult) Threshold() SimilarityScore {
	return r.threshold
}

// SpeakerVerificationResult represents the result of a speaker verification operation
type SpeakerVerificationResult struct {
	speakerID   SpeakerID
	speakerName SpeakerName
	verified    bool
	confidence  SimilarityScore
	threshold   SimilarityScore
}

// NewSpeakerVerificationResult creates a new verification result
func NewSpeakerVerificationResult(
	speakerID SpeakerID,
	speakerName SpeakerName,
	verified bool,
	confidence SimilarityScore,
	threshold SimilarityScore,
) *SpeakerVerificationResult {
	return &SpeakerVerificationResult{
		speakerID:   speakerID,
		speakerName: speakerName,
		verified:    verified,
		confidence:  confidence,
		threshold:   threshold,
	}
}

// SpeakerID returns the speaker's ID being verified
func (r *SpeakerVerificationResult) SpeakerID() SpeakerID {
	return r.speakerID
}

// SpeakerName returns the speaker's name being verified
func (r *SpeakerVerificationResult) SpeakerName() SpeakerName {
	return r.speakerName
}

// Verified returns whether the verification was successful
func (r *SpeakerVerificationResult) Verified() bool {
	return r.verified
}

// Confidence returns the similarity confidence score
func (r *SpeakerVerificationResult) Confidence() SimilarityScore {
	return r.confidence
}

// Threshold returns the similarity threshold used
func (r *SpeakerVerificationResult) Threshold() SimilarityScore {
	return r.threshold
}