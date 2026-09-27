package com.mili.core.webhook.internal.domain;

import com.jayway.jsonpath.InvalidPathException;
import com.jayway.jsonpath.JsonPath;

import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThatThrownBy;

final class Mli05JsonPathProbe {
    @Test void frozenMalformedPathRaisesExpectedException() {
        JsonPath.compile("$.type");
        assertThatThrownBy(() -> JsonPath.compile("$[")).isInstanceOf(InvalidPathException.class);
    }
}
