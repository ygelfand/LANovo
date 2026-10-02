void main() {
	vec2 p = vec2(pixel() / res.y);
	float flash = u[3], rain = u[6], wind = u[7];
	vec2 fp = vec2(u[4], u[5]);
	float y = p.y;
	vec4 land = texture2D(t0, vtex);
	float c = texture2D(t1, fract(p * 0.32 + vec2(u[8], 0.0))).r * 0.55;
	c += texture2D(t1, fract(p * 0.8 + vec2(u[9], u[10]))).g * 0.3;
	c += texture2D(t1, fract(p * 2.1 + vec2(u[11], -u[12]))).r * 0.15;
	float n = clamp((c - 0.5) * 2.8 + 0.5, 0.0, 1.0);
	float d = smoothstep(0.3, 0.7, n + 0.32 - 0.6 * y);
	vec2 dv = (p - fp) * vec2(0.8, 1.3);
	float near = exp(-dot(dv, dv) * 5.0);
	float lit = flash * (near * (0.35 + 1.1 * (1.0 - d) * d + 0.6 * n) + 0.18);
#ifdef LIGHT
	gl_FragColor = vec4(vec3(0.6, 0.65, 0.8) * lit * (1.0 - land.a) * 0.6, 1.0);
#else
	vec3 sky = mix(vec3(0.015, 0.018, 0.028), vec3(0.05, 0.055, 0.07), smoothstep(0.35, 0.8, y));
	sky += vec3(0.06, 0.04, 0.02) * smoothstep(0.55, 0.8, y);
	vec3 body = vec3(0.05, 0.055, 0.075) + vec3(0.11, 0.115, 0.14) * n * n;
	body += vec3(0.05, 0.035, 0.02) * smoothstep(0.3, 0.7, y) * (1.0 - n);
	vec3 clouds = mix(sky, body, d) + vec3(0.75, 0.8, 1.0) * lit;
	vec2 r1 = vec2((p.x + y * wind) * 1.5, y * 0.08 - u[13]);
	float streaks = smoothstep(0.86, 0.97, texture2D(t1, fract(r1)).b) * 0.6;
	vec2 r2 = vec2((p.x + y * wind * 1.3) * 2.6 + 0.37, y * 0.13 - u[14]);
	streaks += smoothstep(0.84, 0.97, texture2D(t1, fract(r2)).b) * 0.35;
	streaks *= smoothstep(0.35, 0.8, rain) * 1.2;
	vec3 rainc = vec3(0.35, 0.4, 0.5) * streaks * (0.25 + 1.2 * flash);
	vec3 ground = land.rgb * (1.0 + 10.0 * flash * (0.4 + near));
	vec3 col = mix(clouds, ground, land.a) + rainc;
	gl_FragColor = vec4(min(col + glow(), 1.0), 1.0);
#endif
}
