#ifdef FEED
void main() {
	float c = texture2D(feed, vgl).r;
	gl_FragColor = vec4(min(c, 1.0) * 0.994, 0.0, 0.0, 1.0);
}
#elif defined(SPLAT)
void main() {
	vec2 d = gl_PointCoord * 2.0 - 1.0;
	float r2 = dot(d, d);
	if (r2 >= 1.0) discard;
#ifdef SPLAT_A
	float w = clamp((1.0 - sqrt(r2)) * vsize * 0.5 + 0.5, 0.0, 1.0);
	gl_FragColor = vec4(d * w, w, 0.0);
#else
	gl_FragColor = vec4(0.0);
#endif
}
#else
void main() {
	vec4 sp = texture2D(splatA, vgl);
	float cov = clamp(sp.b, 0.0, 1.0);
	vec2 n = sp.rg / max(sp.b, 1e-3);
	float clear = texture2D(feed, vgl).r;
	float fog = u[2] * (1.0 - clamp(clear, 0.0, 1.0));
	vec3 sharp = texture2D(t0, vtex).rgb;
	vec3 foggy = texture2D(t1, vtex).rgb;
	vec3 glass = mix(sharp, foggy, fog);
	float rr = min(dot(n, n), 1.0);
	vec3 lens = texture2D(t1, clamp(vtex - n * vec2(0.28, 0.5), 0.0, 1.0)).rgb;
	float rim = smoothstep(0.72, 0.97, rr);
	vec3 drop = lens * (1.08 - 0.75 * rim) + vec3(0.02);
	float h = max(0.0, 1.0 - length(n - vec2(-0.32, -0.45)) * 4.5);
	drop += vec3(1.0) * h * h * 0.85;
	vec3 col = mix(glass, drop, cov);
#ifdef LIGHT
	gl_FragColor = vec4(vec3(h * h * cov * u[4]), 1.0);
#else
	gl_FragColor = vec4(min(col + glow(), 1.0), 1.0);
#endif
}
#endif
