package worker

import (
	"bytes"
	"text/template"
)

func WrapMethodCall(method string) (string, error) {
	tmpl, err := template.New(`sdk`).Parse(`
import * as Contract from "contract";
import { Arguments } from "arguments";
const { argUint32, argUint64, argString, argBytes, argAddress, packedArgumentsEncode, packedArgumentsDecode } = Arguments.Orbs;
import { Types } from "orbs-contract-sdk/v1";

function serializeReturnValue(val) {
	if (typeof val === "undefined") {
		return [argUint32(0), argUint32(0)];
	}

	if (Types.isError(val)) {
		return [argUint32(0), argUint32(1), Types.toArgument(val)];
	}

	return [argUint32(0), argUint32(0), Types.toArgument(val)];
}

V8Worker2.recv(function(msg) {
	const methodCallArguments = packedArgumentsDecode(new Uint8Array(msg)).map(a => a.value);

	let returnValue;
	try {
		if (typeof Contract.{{.method}} === "undefined") {
			throw new Error("method '{{.method}}' not found in contract");
		}

		returnValue = Contract.{{.method}}(...methodCallArguments);
	} catch (e) {
		returnValue = e;
		V8Worker2.print(e);
	}

	const payload = packedArgumentsEncode(serializeReturnValue(returnValue));
	V8Worker2.send(payload.buffer);
});
`)

	if err != nil {
		return "", err
	}

	buf := bytes.NewBufferString("")
	if err = tmpl.Execute(buf, map[string]interface{}{
		"method": method,
	}); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func proxyWriteOnlyMethodCall(sdkObject uint32, sdkMethod uint32, jsParams string, jsWrappedParams string) string {
	tmpl, err := template.New(`sdkProxyMethodCall`).Parse(`
({{.jsParams}}) => {
	V8Worker2.send(packedArgumentsEncode([argUint32({{.sdkObject}}), argUint32({{.sdkMethod}}), {{.jsWrappedParams}}]).buffer);
}`)

	if err != nil {
		panic(err)
	}

	buf := bytes.NewBufferString("")
	tmpl.Execute(buf, map[string]interface{}{
		"sdkObject":       sdkObject,
		"sdkMethod":       sdkMethod,
		"jsParams":        jsParams,
		"jsWrappedParams": jsWrappedParams,
	})

	return buf.String()
}

func proxyReadMethodCall(sdkObject uint32, sdkMethod uint32, jsParams string, jsWrappedParams string, defaultEmptyValue string) string {
	tmpl, err := template.New(`sdkProxyMethodCall`).Parse(`
({{.jsParams}}) => {
	const response = V8Worker2.send(packedArgumentsEncode([argUint32({{.sdkObject}}), argUint32({{.sdkMethod}}), {{.jsWrappedParams}}]).buffer);
	const [ returnType, returnValue ] = packedArgumentsDecode(new Uint8Array(response)).map(a => a.value);

	switch (returnType) {
		case 0: // value
			return packedArgumentsDecode(new Uint8Array(returnValue)).map(a => a.value)[0] || {{.defaultEmptyValue}};
		case 1: // error
			const errValue = packedArgumentsDecode(new Uint8Array(returnValue)).map(a => a.value)[0];
			throw new Error(errValue);
	}

	throw new Error("unsupported return type: " + returnValue);
}`)

	if err != nil {
		panic(err)
	}

	buf := bytes.NewBufferString("")
	tmpl.Execute(buf, map[string]interface{}{
		"sdkObject":         sdkObject,
		"sdkMethod":         sdkMethod,
		"jsParams":          jsParams,
		"jsWrappedParams":   jsWrappedParams,
		"defaultEmptyValue": defaultEmptyValue,
	})

	return buf.String()
}

func getSDKSettings() map[string]interface{} {
	return map[string]interface{}{
		"sdkMethodGetCallerAddress": proxyReadMethodCall(
			SDK_OBJECT_ADDRESS, SDK_METHOD_GET_CALLER_ADDRESS,
			"", "", "undefined",
		),
		"sdkMethodGetSignerAddress": proxyReadMethodCall(
			SDK_OBJECT_ADDRESS, SDK_METHOD_GET_SIGNER_ADDRESS,
			"", "", "undefined",
		),
		"sdkMethodWriteBytes": proxyWriteOnlyMethodCall(
			SDK_OBJECT_STATE, SDK_METHOD_WRITE_BYTES,
			"key, value", "argBytes(key), argBytes(value)",
		),
		"sdkMethodReadBytes": proxyReadMethodCall(
			SDK_OBJECT_STATE, SDK_METHOD_READ_BYTES,
			"key", "argBytes(key)", "new Uint8Array()",
		),
		"sdkMethodWriteUint32": proxyWriteOnlyMethodCall(
			SDK_OBJECT_STATE, SDK_METHOD_WRITE_UINT32,
			"key, value", "argBytes(key), argUint32(value)",
		),
		"sdkMethodReadUint32": proxyReadMethodCall(
			SDK_OBJECT_STATE, SDK_METHOD_READ_UINT32,
			"key", "argBytes(key)", "Uint32(0)",
		),
		"sdkMethodWriteUint64": proxyWriteOnlyMethodCall(
			SDK_OBJECT_STATE, SDK_METHOD_WRITE_UINT64,
			"key, value", "argBytes(key), argUint64(value)",
		),
		"sdkMethodReadUint64": proxyReadMethodCall(
			SDK_OBJECT_STATE, SDK_METHOD_READ_UINT64,
			"key", "argBytes(key)", "Uint64(0)",
		),
		"sdkMethodWriteString": proxyWriteOnlyMethodCall(
			SDK_OBJECT_STATE, SDK_METHOD_WRITE_STRING,
			"key, value", "argBytes(key), argString(value)",
		),
		"sdkMethodReadString": proxyReadMethodCall(
			SDK_OBJECT_STATE, SDK_METHOD_READ_STRING,
			"key", "argBytes(key)", `""`,
		),
		"sdkMethodClear": proxyWriteOnlyMethodCall(
			SDK_OBJECT_STATE, SDK_METHOD_CLEAR,
			"key", "argBytes(key)",
		),
	}
}
