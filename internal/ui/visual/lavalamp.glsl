#define BLOB(o) { vec2 d = vec2((p - vec2(u[o], u[o + 1])) * 0.01); float k = max(0.0, 1.0 - dot(d, d) / (4.0 * u[o + 2] * 0.0001)); field += k * k * k; }

vec3 lamp(highp vec2 p, out float wax) {
	vec4 back = texture2D(t0, vtex);
	wax = 0.0;
	if (back.a <= 0.0) return back.rgb;
	float field = 0.0;
	BLOB(8) BLOB(11) BLOB(14) BLOB(17) BLOB(20) BLOB(23) BLOB(26) BLOB(29) BLOB(32) BLOB(35)
	float t = float((p.y - u[1]) / (u[2] - u[1]));
	vec3 liquid = mix(vec3(0.16, 0.04, 0.3), vec3(0.5, 0.1, 0.4), t) * (0.45 + 0.8 * u[3]);
	liquid += vec3(1.0, 0.45, 0.2) * pow(t, 4.0) * 0.8 * u[3];
	float m = smoothstep(0.38, 0.42, field);
	float core = clamp((field - 0.4) * 1.3, 0.0, 1.0);
	vec3 waxc = mix(vec3(0.85, 0.16, 0.06), vec3(1.0, 0.42, 0.14), core) * (0.6 + 0.55 * u[3]) + vec3(0.25, 0.2, 0.05) * core * t * u[3];
	wax = m;
	vec3 col = mix(liquid, waxc, m);
	return mix(back.rgb, col + back.rgb, back.a);
}

void main() {
	highp vec2 p = pixel() * (u[0] / res.x);
	float wax;
	vec3 col = lamp(p, wax);
#ifdef LIGHT
	gl_FragColor = vec4(col * wax * 0.35, 1.0);
#else
	gl_FragColor = vec4(min(col + glow(), 1.0), 1.0);
#endif
}
