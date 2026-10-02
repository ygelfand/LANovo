vec3 cycle(highp vec2 p, int o) {
	vec3 c = vec3(u[o + 6], u[o + 7], u[o + 8]);
	vec3 col = vec3(0.0);
	highp vec2 dp = vec2(p.x - u[o], (p.y - u[o + 1]) / 0.32);
	float pd = float(length(dp) / u[o + 2]);
	if (pd < 1.0) col += c * u[o + 3] * (1.0 - pd);
	float sd = float(length(p - vec2(u[o], u[o + 4])) / u[o + 5]);
	if (sd < 1.0) {
		vec3 s;
		if (sd < 0.22) s = mix(vec3(1.0), c * 0.95, sd / 0.22);
		else if (sd < 0.55) s = mix(c * 0.95, c * 0.28, (sd - 0.22) / 0.33);
		else s = mix(c * 0.28, vec3(0.0), (sd - 0.55) / 0.45);
		col += s;
	}
	return col;
}

void main() {
	highp float s = res.x / u[20];
	highp vec2 p = pixel() / s;
	vec3 light = linesAt(vgl).rgb + cycle(p, 0) + cycle(p, 10);
#ifdef LIGHT
	gl_FragColor = vec4(light, 1.0);
#else
	gl_FragColor = vec4(min(texture2D(t0, vtex).rgb + light + glow(), 1.0), 1.0);
#endif
}
