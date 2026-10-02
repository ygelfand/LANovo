vec3 scene(highp vec2 p) {
	highp vec2 c = vec2(u[0], u[1]);
	highp float rs = u[2], H = u[4];
	float lv = u[3];
	vec3 base = vec3(u[5], u[6], u[7]), hot = vec3(u[8], u[9], u[10]);
	highp float d = length(p - c);
	vec3 col = vec3(2.0, 3.0, 10.0) / 255.0;
	col = mix(col, base, (0.1 + 0.14 * lv) * max(1.0 - float(d / (H * 0.75)), 0.0));
	float k = float(d / (rs * 0.75));
	if (k < 1.0) col += mix(hot, base, k) * (0.1 + 0.22 * lv) * (1.0 - k);
	col += linesAt(vgl).rgb;
	return min(col, 1.0);
}

void main() {
	highp vec2 p = pixel() * (u[11] / res.x);
#ifdef LIGHT
	gl_FragColor = vec4(scene(p), 1.0);
#else
	gl_FragColor = vec4(min(scene(p) + glow(), 1.0), 1.0);
#endif
}
