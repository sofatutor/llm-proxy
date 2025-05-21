package token

import (
	"testing"
	"time"
)

func TestCalculateExpiration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		wantNil  bool
	}{
		{
			name:     "One hour expiration",
			duration: OneHour,
			wantNil:  false,
		},
		{
			name:     "One day expiration",
			duration: OneDay,
			wantNil:  false,
		},
		{
			name:     "No expiration (zero)",
			duration: NoExpiration,
			wantNil:  true,
		},
		{
			name:     "No expiration (negative)",
			duration: -1 * time.Hour,
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			expiration := CalculateExpiration(tt.duration)

			if tt.wantNil {
				if expiration != nil {
					t.Errorf("CalculateExpiration() = %v, want nil", expiration)
				}
				return
			}

			if expiration == nil {
				t.Errorf("CalculateExpiration() = nil, want non-nil")
				return
			}

			// Check expiration time is roughly now + duration (allowing 1 second tolerance)
			expected := now.Add(tt.duration)
			diff := expected.Sub(*expiration)
			if diff < -1*time.Second || diff > 1*time.Second {
				t.Errorf("CalculateExpiration() time = %v, want approximately %v (diff: %v)", *expiration, expected, diff)
			}
		})
	}
}

func TestCalculateExpirationFrom(t *testing.T) {
	tests := []struct {
		name      string
		startTime time.Time
		duration  time.Duration
		wantNil   bool
	}{
		{
			name:      "One hour from start",
			startTime: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			duration:  OneHour,
			wantNil:   false,
		},
		{
			name:      "One day from start",
			startTime: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			duration:  OneDay,
			wantNil:   false,
		},
		{
			name:      "No expiration (zero)",
			startTime: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			duration:  NoExpiration,
			wantNil:   true,
		},
		{
			name:      "No expiration (negative)",
			startTime: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			duration:  -1 * time.Hour,
			wantNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expiration := CalculateExpirationFrom(tt.startTime, tt.duration)

			if tt.wantNil {
				if expiration != nil {
					t.Errorf("CalculateExpirationFrom() = %v, want nil", expiration)
				}
				return
			}

			if expiration == nil {
				t.Errorf("CalculateExpirationFrom() = nil, want non-nil")
				return
			}

			// Check expiration time is exactly startTime + duration
			expected := tt.startTime.Add(tt.duration)
			if !expiration.Equal(expected) {
				t.Errorf("CalculateExpirationFrom() = %v, want %v", *expiration, expected)
			}
		})
	}
}

func TestValidateExpiration(t *testing.T) {
	now := time.Now()
	pastTime := now.Add(-1 * time.Hour)
	futureTime := now.Add(1 * time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		wantErr   bool
	}{
		{
			name:      "Nil expiration (no expiry)",
			expiresAt: nil,
			wantErr:   false,
		},
		{
			name:      "Future expiration",
			expiresAt: &futureTime,
			wantErr:   false,
		},
		{
			name:      "Past expiration",
			expiresAt: &pastTime,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExpiration(tt.expiresAt)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExpiration() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsExpired(t *testing.T) {
	now := time.Now()
	pastTime := now.Add(-1 * time.Hour)
	futureTime := now.Add(1 * time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		want      bool
	}{
		{
			name:      "Nil expiration (no expiry)",
			expiresAt: nil,
			want:      false,
		},
		{
			name:      "Future expiration",
			expiresAt: &futureTime,
			want:      false,
		},
		{
			name:      "Past expiration",
			expiresAt: &pastTime,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsExpired(tt.expiresAt); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTimeUntilExpiration(t *testing.T) {
	now := time.Now()
	pastTime := now.Add(-1 * time.Hour)
	futureTime := now.Add(1 * time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		want      time.Duration
		check     func(time.Duration) bool
	}{
		{
			name:      "Nil expiration (no expiry)",
			expiresAt: nil,
			check:     func(d time.Duration) bool { return d > 1000*time.Hour }, // Very long duration
		},
		{
			name:      "Future expiration",
			expiresAt: &futureTime,
			check:     func(d time.Duration) bool { return d > 0 && d <= 1*time.Hour },
		},
		{
			name:      "Past expiration",
			expiresAt: &pastTime,
			check:     func(d time.Duration) bool { return d == 0 },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TimeUntilExpiration(tt.expiresAt)
			if !tt.check(got) {
				t.Errorf("TimeUntilExpiration() = %v, failed check", got)
			}
		})
	}
}

func TestExpiresWithin(t *testing.T) {
	now := time.Now()
	pastTime := now.Add(-1 * time.Hour)
	soon := now.Add(30 * time.Minute)
	later := now.Add(2 * time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		duration  time.Duration
		want      bool
	}{
		{
			name:      "Nil expiration (no expiry)",
			expiresAt: nil,
			duration:  1 * time.Hour,
			want:      false,
		},
		{
			name:      "Expires soon (within duration)",
			expiresAt: &soon,
			duration:  1 * time.Hour,
			want:      true,
		},
		{
			name:      "Expires later (outside duration)",
			expiresAt: &later,
			duration:  1 * time.Hour,
			want:      false,
		},
		{
			name:      "Already expired",
			expiresAt: &pastTime,
			duration:  1 * time.Hour,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExpiresWithin(tt.expiresAt, tt.duration); got != tt.want {
				t.Errorf("ExpiresWithin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatExpirationTime(t *testing.T) {
	now := time.Now()
	pastTime := now.Add(-1 * time.Hour)
	futureTime := now.Add(1 * time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		want      string
	}{
		{
			name:      "Nil expiration (no expiry)",
			expiresAt: nil,
			want:      "Never expires",
		},
		{
			name:      "Future expiration",
			expiresAt: &futureTime,
			want:      futureTime.Format(time.RFC3339),
		},
		{
			name:      "Past expiration",
			expiresAt: &pastTime,
			want:      "Expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatExpirationTime(tt.expiresAt); got != tt.want {
				t.Errorf("FormatExpirationTime() = %v, want %v", got, tt.want)
			}
		})
	}
}