package com.mili.core.webhook.internal.infrastructure.http;

import com.mili.core.webhook.api.InboundEventSummary;
import com.mili.core.webhook.internal.infrastructure.persistence.InboundEventRepository;
import org.junit.jupiter.api.Test;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;

import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

class Mli02AcceptanceTest {
    @Test void rejectsMalformedCursorsAndBuildsStableNextCursor() throws Exception {
        var repository = mock(InboundEventRepository.class);
        var service = (com.mili.core.webhook.WebhookMonitoringService) java.lang.reflect.Proxy.newProxyInstance(
                getClass().getClassLoader(),
                new Class<?>[] {com.mili.core.webhook.WebhookMonitoringService.class},
                (proxy, method, args) -> {
                    if (!method.getName().equals("listEvents")) return null;
                    String cursor = (String) args[3];
                    if (cursor == null || cursor.isBlank()) return repository.findSummaries((UUID) args[0], (String) args[1], (String) args[2], null, null);
                    int separator = cursor.lastIndexOf('|');
                    if (separator < 1 || separator == cursor.length() - 1) throw new IllegalArgumentException("Malformed page cursor");
                    long epoch;
                    try { epoch = Long.parseLong(cursor.substring(0, separator)); }
                    catch (NumberFormatException exception) { throw new IllegalArgumentException("Malformed page cursor", exception); }
                    return repository.findSummaries((UUID) args[0], (String) args[1], (String) args[2], Instant.ofEpochMilli(epoch), cursor.substring(separator + 1));
                });
        var mvc = MockMvcBuilders.standaloneSetup(new WebhookMonitoringController(service)).build();
        var tenant = UUID.randomUUID();
        mvc.perform(get("/v1/inbound-events").param("tenantId", tenant.toString())).andExpect(status().isOk());
        verify(repository).findSummaries(eq(tenant), isNull(), isNull(), isNull(), isNull());
        mvc.perform(get("/v1/inbound-events").param("tenantId", tenant.toString()).param("pageAfter", " ")).andExpect(status().isOk());
        for (String cursor : List.of("abc|evt-2", "1730000000123", "1730000000123|", "9223372036854775808|evt-2")) {
            try { mvc.perform(get("/v1/inbound-events").param("tenantId", tenant.toString()).param("pageAfter", cursor)).andExpect(status().isBadRequest()); }
            catch (jakarta.servlet.ServletException failure) { assertThat(failure.getCause()).isInstanceOf(IllegalArgumentException.class); }
        }
        verify(repository, times(2)).findSummaries(eq(tenant), isNull(), isNull(), isNull(), isNull());

        Instant lastTime = Instant.parse("2026-09-27T12:00:00Z");
        var firstPage = new ArrayList<InboundEventSummary>();
        for (int i = 0; i < 50; i++) firstPage.add(new InboundEventSummary("evt-" + i, tenant, UUID.randomUUID(), "type", "PROCESSED", lastTime.minusSeconds(i)));
        when(repository.findSummaries(eq(tenant), isNull(), isNull(), isNull(), isNull())).thenReturn(firstPage);
        String body = mvc.perform(get("/v1/inbound-events").param("tenantId", tenant.toString())).andExpect(status().isOk()).andReturn().getResponse().getContentAsString();
        String cursor = lastTime.minusSeconds(49).toEpochMilli() + "|evt-49";
        assertThat(body).contains(cursor);
        when(repository.findSummaries(eq(tenant), isNull(), isNull(), eq(lastTime.minusSeconds(49)), eq("evt-49"))).thenReturn(List.of());
        mvc.perform(get("/v1/inbound-events").param("tenantId", tenant.toString()).param("pageAfter", cursor)).andExpect(status().isOk());
        verify(repository).findSummaries(eq(tenant), isNull(), isNull(), eq(lastTime.minusSeconds(49)), eq("evt-49"));
    }
}











