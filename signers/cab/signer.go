/*
 * Copyright (c) SAS Institute Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cab

// Sign Microsoft cabinet files

import (
	"io"
	"os"

	"gerrit-pdt.unx.sas.com/tools/relic.git/lib/authenticode"
	"gerrit-pdt.unx.sas.com/tools/relic.git/lib/cabfile"
	"gerrit-pdt.unx.sas.com/tools/relic.git/lib/certloader"
	"gerrit-pdt.unx.sas.com/tools/relic.git/lib/magic"
	"gerrit-pdt.unx.sas.com/tools/relic.git/signers"
	"gerrit-pdt.unx.sas.com/tools/relic.git/signers/pkcs"
)

var CabSigner = &signers.Signer{
	Name:      "cab",
	Magic:     magic.FileTypeCAB,
	CertTypes: signers.CertTypeX509,
	Sign:      sign,
	Verify:    verify,
}

func init() {
	signers.Register(CabSigner)
}

func sign(r io.Reader, cert *certloader.Certificate, opts signers.SignOpts) ([]byte, error) {
	digest, err := cabfile.Digest(r, opts.Hash)
	if err != nil {
		return nil, err
	}
	psd, err := authenticode.SignCabImprint(digest.Imprint, opts.Hash, cert.Signer(), cert.Chain())
	if err != nil {
		return nil, err
	}
	blob, err := pkcs.Timestamp(psd, cert, opts, true)
	if err != nil {
		return nil, err
	}
	patch := digest.MakePatch(blob)
	return opts.SetBinPatch(patch)
}

func verify(f *os.File, opts signers.VerifyOpts) ([]*signers.Signature, error) {
	sig, err := authenticode.VerifyCab(f, opts.NoDigests)
	if err != nil {
		return nil, err
	}
	return []*signers.Signature{&signers.Signature{
		Hash:          sig.HashFunc,
		X509Signature: &sig.TimestampedSignature,
	}}, nil
}
