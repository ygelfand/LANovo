float rounded(highp vec2 p, highp vec2 at, highp vec2 size, float r) {
	highp vec2 e = max(max(at + r - p, p - (at + size - r)), 0.0);
	return clamp(r - float(length(e)) + 0.5, 0.0, 1.0);
}

void main() {
#ifdef FEED
	gl_FragColor = vec4(min(texture2D(feed, vgl).rgb * u[0] + linesAt(vgl).rgb, 1.0), 1.0);
#else
#ifdef LIGHT
	gl_FragColor = texture2D(feed, vgl);
#else
	highp vec2 p = pixel();
	vec3 col = texture2D(t0, vtex).rgb;
	float inside = rounded(p, vec2(u[1], u[2]), vec2(u[3], u[4]), u[5]);
	if (inside > 0.0) {
		vec3 s = min(col + texture2D(feed, vgl).rgb, 1.0);
		vec4 g = texture2D(t1, vtex);
		s = g.rgb + s * (1.0 - g.a);
		col = mix(col, s, inside);
	}
	gl_FragColor = vec4(min(col + glow(), 1.0), 1.0);
#endif
#endif
}
