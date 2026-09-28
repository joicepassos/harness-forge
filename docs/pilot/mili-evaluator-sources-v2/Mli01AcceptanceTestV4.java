package com.mili.core.webhook.pilot;

import com.mili.core.tenant.TenantContext;
import com.mili.core.tenant.TenantId;
import com.mili.core.webhook.WebhookMonitoringService;
import com.mili.core.webhook.api.InboundEventDetail;
import com.mili.core.webhook.api.InboundEventSummary;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.invocation.InvocationOnMock;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import java.lang.reflect.Constructor;
import java.time.Instant;
import java.util.Arrays;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.TreeMap;
import java.util.UUID;
import java.util.concurrent.CopyOnWriteArrayList;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.withSettings;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;

/**
 * Independent no-database acceptance checks for authenticated tenant scoping.
 *
 * The production monitoring controller is exercised through Spring MVC's
 * in-process request mapping. Its service boundary is a deterministic fake
 * backed by two tenant-owned events, so no application context, database,
 * broker, network service, wall clock, or Testcontainers is involved.
 */
class Mli01AcceptanceTestV4 {

    private static final String CONTROLLER =
            "com.mili.core.webhook.internal.infrastructure.http.WebhookMonitoringController";
    private static final UUID TENANT_A = UUID.fromString("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa");
    private static final UUID TENANT_B = UUID.fromString("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb");
    private static final String EVENT_A = "event-a";
    private static final String EVENT_B = "event-b";
    private static final String UNKNOWN_EVENT = "event-unknown";
    private static final Instant RECEIVED_AT = Instant.parse("2025-01-02T03:04:05Z");

    private final List<String> mutations = new CopyOnWriteArrayList<>();
    private final List<String> resetAttempts = new CopyOnWriteArrayList<>();
    private Object inboundEventRepository;
    private WebhookMonitoringService monitoringService;
    private MockMvc mvc;

    @BeforeEach
    void setUp() throws Exception {
        mutations.clear();
        resetAttempts.clear();
        Class<?> repositoryType = Class.forName(
                "com.mili.core.webhook.internal.infrastructure.persistence.InboundEventRepository");
        inboundEventRepository = mock(repositoryType, withSettings().defaultAnswer(this::fakeRepository));
        monitoringService = constructMonitoringService(inboundEventRepository);
        Object controller = constructController(monitoringService);
        mvc = MockMvcBuilders.standaloneSetup(controller)
                .setControllerAdvice(loadAdvice(
                        "com.mili.core.webhook.internal.infrastructure.http.GlobalExceptionHandler",
                        "com.mili.core.tenant.internal.TenantExceptionHandler"))
                .build();
        authenticateAs(TENANT_A);
    }

    @AfterEach
    void clearSecurityContext() {
        SecurityContextHolder.clearContext();
    }

    @Test
    void listUsesAuthenticatedTenantAndCannotBeSwitchedByQueryParameter() throws Exception {
        MvcResult ownList = mvc.perform(get("/v1/inbound-events"))
                        .andReturn();
        assertThat(ownList.getResponse().getStatus()).isEqualTo(200);
        assertThat(ownList.getResponse().getContentAsString()).contains(EVENT_A).doesNotContain(EVENT_B);
        assertRepositoryCallScopedTo(TENANT_A, "findSummaries");

        // The legacy tenantId may be ignored or rejected, but it may never
        // redirect the authenticated request to tenant B.
        MvcResult forgedList = mvc.perform(get("/v1/inbound-events").param("tenantId", TENANT_B.toString()))
                .andReturn();
        int status = forgedList.getResponse().getStatus();
        assertThat(status == 200 || status == 400 || status == 403).isTrue();
        if (status == 200) {
            assertThat(forgedList.getResponse().getContentAsString()).contains(EVENT_A).doesNotContain(EVENT_B);
            assertRepositoryCallScopedTo(TENANT_A, "findSummaries");
        } else {
            assertThat(invocationsOf("findSummaries")).noneSatisfy(invocation ->
                    assertThat(hasTenant(invocation, TENANT_B)).isTrue());
        }
    }

    @Test
    void detailForForeignEventIsSameNotFoundAsUnknownAndOwnEventRemainsVisible() throws Exception {
        MvcResult foreign = mvc.perform(get("/v1/inbound-events/{id}", EVENT_B)).andReturn();
        MvcResult unknown = mvc.perform(get("/v1/inbound-events/{id}", UNKNOWN_EVENT)).andReturn();
        assertIndistinguishableFromUnknown(foreign, unknown);
        assertThat(foreign.getResponse().getContentAsString()).doesNotContain(EVENT_B);

        MvcResult own = mvc.perform(get("/v1/inbound-events/{id}", EVENT_A)).andReturn();
        assertThat(own.getResponse().getStatus()).isEqualTo(200);
        assertThat(own.getResponse().getContentAsString()).contains(EVENT_A);
        assertRepositoryCallScopedTo(TENANT_A, "findDetailByTenantIdAndId", EVENT_A);
    }

    @Test
    void reprocessForForeignEventIsSameNotFoundAsUnknownAndNeverMutatesIt() throws Exception {
        MvcResult foreign = mvc.perform(post("/v1/inbound-events/{id}/reprocess", EVENT_B)).andReturn();
        MvcResult unknown = mvc.perform(post("/v1/inbound-events/{id}/reprocess", UNKNOWN_EVENT)).andReturn();
        assertIndistinguishableFromUnknown(foreign, unknown);
        assertThat(foreign.getResponse().getContentAsString()).doesNotContain(EVENT_B);
        assertThat(mutations).doesNotContain(EVENT_B);
        assertThat(resetAttempts).doesNotContain(EVENT_B);
    }

    private Object fakeRepository(InvocationOnMock invocation) {
        String method = invocation.getMethod().getName();
        Object[] args = invocation.getArguments();
        boolean scopedToA = hasTenant(invocation, TENANT_A);
        boolean scopedToB = hasTenant(invocation, TENANT_B);
        return switch (method) {
            case "findSummaries" -> scopedToA ? List.of(summary(EVENT_A, TENANT_A))
                    : scopedToB ? List.of(summary(EVENT_B, TENANT_B))
                    : List.of(summary(EVENT_B, TENANT_B));
            case "findDetailByTenantIdAndId", "findDetailById" -> {
                String id = eventId(args);
                if (EVENT_A.equals(id) && scopedToA) yield Optional.of(detail(EVENT_A, TENANT_A));
                if (EVENT_A.equals(id) && !hasAnyTenant(invocation)) yield Optional.of(detail(EVENT_A, TENANT_A));
                if (EVENT_B.equals(id) && (scopedToB || !hasAnyTenant(invocation)))
                    yield Optional.of(detail(EVENT_B, TENANT_B));
                yield Optional.empty();
            }
            case "resetToPending" -> {
                String id = eventId(args);
                if (EVENT_B.equals(id)) resetAttempts.add(id);
                if (EVENT_B.equals(id) && (scopedToB || !hasAnyTenant(invocation))) {
                    mutations.add(id);
                }
                yield defaultValue(invocation.getMethod().getReturnType());
            }
            default -> throw new AssertionError("Unexpected inbound-event repository call: " + method);
        };
    }

    private static InboundEventSummary summary(String id, UUID tenantId) {
        return new InboundEventSummary(id, tenantId, UUID.nameUUIDFromBytes((id + "-webhook").getBytes()),
                "order.created", "FAILED", RECEIVED_AT);
    }

    private static InboundEventDetail detail(String id, UUID tenantId) {
        return new InboundEventDetail(id, tenantId, UUID.nameUUIDFromBytes((id + "-webhook").getBytes()),
                "order.created", "{\"type\":\"order.created\"}", null, "FAILED", "timeout",
                RECEIVED_AT, null);
    }

    private void assertRepositoryCallScopedTo(UUID tenant, String method, String... expectedIds) {
        List<InvocationOnMock> matching = invocationsOf(method).stream()
                .filter(invocation -> expectedIds.length == 0 || Arrays.stream(expectedIds)
                        .anyMatch(id -> Arrays.asList(invocation.getArguments()).contains(id)))
                .toList();
        assertThat(matching).as("%s service invocation(s) for %s", method, Arrays.toString(expectedIds))
                .isNotEmpty()
                .allSatisfy(invocation -> assertThat(hasTenant(invocation, tenant))
                        .as("authenticated tenant must be passed to %s", method).isTrue());
    }

    private List<InvocationOnMock> invocationsOf(String methodName) {
        return org.mockito.Mockito.mockingDetails(inboundEventRepository).getInvocations().stream()
                .filter(invocation -> methodName.equals("findDetailByTenantIdAndId") ? List.of("findDetailByTenantIdAndId", "findDetailById").contains(invocation.getMethod().getName()) : invocation.getMethod().getName().equals(methodName))
                .map(invocation -> (InvocationOnMock) invocation)
                .toList();
    }

    private static void assertIndistinguishableFromUnknown(MvcResult actual, MvcResult unknown) throws Exception {
        assertThat(unknown.getResponse().getStatus()).isEqualTo(404);
        assertThat(actual.getResponse().getStatus()).isEqualTo(unknown.getResponse().getStatus());
        assertThat(actual.getResponse().getContentType()).isEqualTo(unknown.getResponse().getContentType());
        assertThat(actual.getResponse().getContentAsString()).isEqualTo(unknown.getResponse().getContentAsString());
        assertThat(stableHeaders(actual.getResponse())).isEqualTo(stableHeaders(unknown.getResponse()));
    }

    private static Map<String, List<String>> stableHeaders(
            org.springframework.mock.web.MockHttpServletResponse response) {
        Set<String> perRequestHeaders = Set.of(
                "date", "x-request-id", "request-id", "x-correlation-id", "traceparent");
        Map<String, List<String>> headers = new TreeMap<>(String.CASE_INSENSITIVE_ORDER);
        for (String name : response.getHeaderNames()) {
            if (!perRequestHeaders.contains(name.toLowerCase())) {
                headers.put(name, List.copyOf(response.getHeaders(name)));
            }
        }
        return headers;
    }

    private static String eventId(Object[] args) {
        return Arrays.stream(args).filter(String.class::isInstance).map(String.class::cast)
                .filter(value -> value.startsWith("event-")).findFirst().orElse("");
    }

    private static boolean hasTenant(InvocationOnMock invocation, UUID tenant) {
        return Arrays.stream(invocation.getArguments()).anyMatch(argument ->
                tenant.equals(argument) || argument instanceof TenantId tenantId && tenant.equals(tenantId.value()));
    }

    private static boolean hasAnyTenant(InvocationOnMock invocation) {
        return hasTenant(invocation, TENANT_A) || hasTenant(invocation, TENANT_B);
    }

    private static Object defaultValue(Class<?> type) {
        if (type.equals(void.class)) return null;
        if (!type.isPrimitive()) return null;
        if (type.equals(boolean.class)) return false;
        if (type.equals(char.class)) return '\0';
        if (type.equals(byte.class)) return (byte) 0;
        if (type.equals(short.class)) return (short) 0;
        if (type.equals(int.class)) return 0;
        if (type.equals(long.class)) return 0L;
        if (type.equals(float.class)) return 0F;
        if (type.equals(double.class)) return 0D;
        throw new AssertionError("Unsupported primitive return type " + type);
    }

    private static Object[] loadAdvice(String... classNames) throws Exception {
        Object[] advice = new Object[classNames.length];
        for (int index = 0; index < classNames.length; index++) {
            Class<?> type = Class.forName(classNames[index]);
            Constructor<?> constructor = type.getDeclaredConstructor();
            constructor.setAccessible(true);
            advice[index] = constructor.newInstance();
        }
        return advice;
    }

    private static void authenticateAs(UUID tenant) {
        var context = SecurityContextHolder.createEmptyContext();
        context.setAuthentication(new UsernamePasswordAuthenticationToken(
                new TenantId(tenant), null, List.of()));
        SecurityContextHolder.setContext(context);
    }

    private static Object constructController(WebhookMonitoringService service) throws Exception {
        Class<?> controllerType = Class.forName(CONTROLLER);
        for (Constructor<?> constructor : Arrays.stream(controllerType.getDeclaredConstructors())
                .sorted((left, right) -> Integer.compare(left.getParameterCount(), right.getParameterCount()))
                .toList()) {
            Object[] arguments = new Object[constructor.getParameterCount()];
            boolean supported = true;
            for (int index = 0; index < arguments.length; index++) {
                Class<?> parameter = constructor.getParameterTypes()[index];
                if (parameter.isInstance(service)) arguments[index] = service;
                else if (parameter.equals(TenantContext.class)) arguments[index] = new TenantContext();
                else {
                    supported = false;
                    break;
                }
            }
            if (supported) {
                constructor.setAccessible(true);
                return constructor.newInstance(arguments);
            }
        }
        throw new AssertionError("Cannot construct monitoring controller with service and TenantContext dependencies");
    }

    private static WebhookMonitoringService constructMonitoringService(Object repository) throws Exception {
        Class<?> type = Class.forName(
                "com.mili.core.webhook.internal.application.monitoring.WebhookMonitoringServiceImpl");
        for (Constructor<?> constructor : type.getDeclaredConstructors()) {
            if (constructor.getParameterCount() == 1 && constructor.getParameterTypes()[0].isInstance(repository)) {
                constructor.setAccessible(true);
                return (WebhookMonitoringService) constructor.newInstance(repository);
            }
        }
        throw new AssertionError("Cannot construct production monitoring service with inbound-event repository");
    }
}
