package oci

import "testing"

func TestGetRegRepoTag(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		expReg  string
		expRepo string
		expTag  string
		expErr  error
	}{
		{
			name:    "valid/all_present",
			input:   "docker.io/repo:v1",
			expReg:  "docker.io",
			expRepo: "docker.io/repo",
			expTag:  "v1",
		},
		{
			name:    "valid/no_registry",
			input:   "myrepo:new",
			expReg:  DefaultRegistry,
			expRepo: DefaultRegistry + "/myrepo",
			expTag:  "new",
		},
		{
			name:    "valid/no_tag",
			input:   "localhost:5000/myrepo",
			expReg:  "localhost:5000",
			expRepo: "localhost:5000/myrepo",
			expTag:  DefaultTag,
		},
		{
			name:    "valid/no_reg_no_tag",
			input:   "onlyrepo",
			expReg:  DefaultRegistry,
			expRepo: DefaultRegistry + "/onlyrepo",
			expTag:  DefaultTag,
		},
		{
			name:   "invalid/empty",
			input:  "",
			expErr: ErrInvalidImage,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reg, repo, tag, err := getRegRepoTag(tc.input)
			if err != nil {
				if tc.expErr == nil {
					t.Errorf("got error %s, expect no error", err)
				}
				if err != tc.expErr {
					t.Errorf("got error %s, expect error %s", err, tc.expErr)
				}
			} else if tc.expErr != nil {
				t.Errorf("got no error, expect error %s", tc.expErr)
			}

			if reg != tc.expReg {
				t.Errorf("registry: got %s, expect %s", reg, tc.expReg)
			}
			if repo != tc.expRepo {
				t.Errorf("repository: got %s, expect %s", repo, tc.expRepo)
			}
			if tag != tc.expTag {
				t.Errorf("tag: got %s, expect %s", tag, tc.expTag)
			}

		})
	}
}
