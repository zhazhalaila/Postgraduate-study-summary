import ecdsa
import json

with open('replica.json', 'w') as fp:
    l = []
    for i in range(64):
        d = {}
        sk = ecdsa.SigningKey.generate(curve=ecdsa.SECP256k1)
        vk = sk.verifying_key
        d['signingKey'] = sk.to_string().hex()
        d['verifyingKey'] = vk.to_string().hex()
        l.append(d)
    print(l)
    json.dump(l, fp)
