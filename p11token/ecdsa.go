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

package p11token

import (
	"crypto"
	"crypto/ecdsa"
	"encoding/asn1"
	"errors"

	"gerrit-pdt.unx.sas.com/tools/relic.git/lib/x509tools"
	"github.com/miekg/pkcs11"
)

// Convert token ECDSA public key to *ecdsa.PublicKey
func (key *Key) toEcdsaKey() (crypto.PublicKey, error) {
	ecparams := key.token.getAttribute(key.pub, pkcs11.CKA_EC_PARAMS)
	ecpoint := key.token.getAttribute(key.pub, pkcs11.CKA_EC_POINT)
	if len(ecparams) == 0 || len(ecpoint) == 0 {
		return nil, errors.New("Unable to retrieve ECDSA public key")
	}
	curve, err := x509tools.CurveByDer(ecparams)
	if err != nil {
		return nil, err
	}
	x, y := x509tools.DerToPoint(curve.Curve, ecpoint)
	if x == nil || y == nil {
		return nil, errors.New("Invalid elliptic curve point")
	}
	eckey := &ecdsa.PublicKey{Curve: curve.Curve, X: x, Y: y}
	return eckey, nil
}

// Sign a digest using token ECDSA private key
func (key *Key) signECDSA(digest []byte) (der []byte, err error) {
	mech := pkcs11.NewMechanism(pkcs11.CKM_ECDSA, nil)
	err = key.token.ctx.SignInit(key.token.sh, []*pkcs11.Mechanism{mech}, key.priv)
	if err != nil {
		return nil, err
	}
	sig, err := key.token.ctx.Sign(key.token.sh, digest)
	if err != nil {
		return nil, err
	}
	sigBytes := len(sig) / 2
	r := bytesToBig(sig[:sigBytes])
	s := bytesToBig(sig[sigBytes:])
	return asn1.Marshal(x509tools.EcdsaSignature{r, s})
}

// Generate ECDSA-specific public and private key attributes from a PrivateKey
func ecdsaImportAttrs(priv *ecdsa.PrivateKey) (pub_attrs, priv_attrs []*pkcs11.Attribute, err error) {
	curve, err := x509tools.CurveByCurve(priv.Curve)
	if err != nil {
		return nil, nil, err
	}
	pub_attrs = []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_EC_PARAMS, curve.ToDer()),
		pkcs11.NewAttribute(pkcs11.CKA_EC_POINT, x509tools.PointToDer(&priv.PublicKey)),
	}
	priv_attrs = []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_EC_PARAMS, curve.ToDer()),
		pkcs11.NewAttribute(pkcs11.CKA_VALUE, priv.D.Bytes()),
	}
	return
}

// Generate ECDSA-specific public attributes to generate an ECSDA key in the token
func ecdsaGenerateAttrs(bits uint) ([]*pkcs11.Attribute, *pkcs11.Mechanism, error) {
	curve, err := x509tools.CurveByBits(bits)
	if err != nil {
		return nil, nil, err
	}
	pubAttrs := []*pkcs11.Attribute{pkcs11.NewAttribute(pkcs11.CKA_EC_PARAMS, curve.ToDer())}
	mech := pkcs11.NewMechanism(pkcs11.CKM_EC_KEY_PAIR_GEN, nil)
	return pubAttrs, mech, nil
}
