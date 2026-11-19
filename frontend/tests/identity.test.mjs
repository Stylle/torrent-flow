import { test } from 'node:test';
import assert from 'node:assert/strict';
import { webcrypto } from 'node:crypto';
import { createID } from '../src/identity.js';
test('LAN HTTP fallback produces unique valid UUIDs without randomUUID',()=>{
 const source={getRandomValues:webcrypto.getRandomValues.bind(webcrypto)};
 const values=new Set(Array.from({length:1000},()=>createID(source)));
 assert.equal(values.size,1000);
 for(const value of values)assert.match(value,/^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$/);
});
test('secure context uses native UUID',()=>assert.equal(createID({randomUUID:()=> 'native-uuid'}),'native-uuid'));
test('never silently falls back to insecure Math.random',()=>assert.throws(()=>createID({}),/cryptographic randomness/));
