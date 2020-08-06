// https://bl.ocks.org/larsenmtl/e3b8b7c2ca4787f77d78f58d41c3da91


(async () => {
	const margin = {top: 10, right: 30, bottom: 30, left: 60},
		width = 1020 - margin.left - margin.right,
		height = 550 - margin.top - margin.bottom;

	const svg = d3.select("#my_dataviz")
		.append("svg")
		.attr("width", width + margin.left + margin.right)
		.attr("height", height + margin.top + margin.bottom)
		.append("g")
		.attr("transform", `translate(${margin.left}, ${margin.top})`);

	const serverNames = [
		//"Hive",
		"Zentria",
		"Kännukas",
		"Throne",
		"Pame",
		"ohmyremi",
		"Pelar",
		"Ordu",
		"Relaxx",
	];

	const data = await fetch("http://127.0.0.1:5000/mctrack_servers?order=timestamp.asc")
		.then(res => res.json())
		.then(res => {
			return res
				.filter((row) => serverNames.includes(row.name))
				.map((row) => ({...row, timestamp: d3.isoParse(row.timestamp)})); 
		});


	const color = d3.scaleOrdinal(d3.schemeCategory10);
	color.domain(serverNames);

	const servers = color.domain().map((name) => {
		return {
			name: name,
			values: data.filter((row) => row.name === name),
		};
	});

	// Set up legend
	const legend = svg.selectAll('g')
		.data(servers)
		.enter()
		.append('g')
		.attr('class', 'legend');

	legend.append('rect')
		.attr('x', width - 70)
		.attr('y', (d, i) => i * 20)
		.attr('width', 10)
		.attr('height', 10)
		.style('fill', (d) => color(d.name));

	legend.append('text')
		.attr('x', width - 58)
		.attr('y', (d, i) => (i * 20) + 9)
		.text((d) => d.name);

	// Set up axes
	const x = d3.scaleTime()
		.domain(d3.extent(data, (d) => d.timestamp))
		.range([ 0, width ]);

	const y = d3.scaleLinear()
		.domain([
			d3.min(data, (d) => +d.online),
			d3.max(data, (d) => +d.online)
		])
		.range([ height, 0 ]);

	const xAxis = d3.axisBottom(x);	

	const yAxis = d3.axisLeft(y)
		.tickFormat(d3.format("d"));

	svg.append("g")
		.attr("transform", `translate(0, ${height})`)
		.call(xAxis);

	svg.append("g")
		.call(yAxis);

	const line = d3.line()
		.curve(d3.curveBasis)
		.x((d) => x(d.timestamp))
		.y((d) => y(d.online));

	const server = svg.selectAll('.server')
		.data(servers)
		.enter()
		.append('g')
		.attr('class', 'server');

	server.append('path')
		.attr('class', 'line')
		.style('fill', 'none')
		.attr('d', (d) => line(d.values))
		.style('stroke', (d) => color(d.name));

	// Mouse over stuff
	const mouseG = svg.append("g")
		.attr("class", "mouse-over-effects");

	mouseG.append("path") // this is the black vertical line to follow mouse
		.attr("class", "mouse-line")
		.style("stroke", "black")
		.style("stroke-width", "1px")
		.style("opacity", "0");

	const lines = document.querySelectorAll('.line');

	const mousePerLine = mouseG.selectAll('.mouse-per-line')
		.data(servers)
		.enter()
		.append("g")
		.attr("class", "mouse-per-line");

	mousePerLine.append("circle")
		.attr("r", 7)
		.style("stroke", (d) => color(d.name))
		.style("fill", "none")
		.style("stroke-width", "1px")
		.style("opacity", "0");

	mousePerLine.append("text")
		.attr("transform", "translate(10,3)");

	mouseG.append('svg:rect') // append a rect to catch mouse movements on canvas
		.attr('width', width) // can't catch mouse events on a g element
		.attr('height', height)
		.attr('fill', 'none')
		.attr('pointer-events', 'all')
		.on('mouseout', () => { // on mouse out hide line, circles and text
			d3.select(".mouse-line")
				.style("opacity", "0");
			d3.selectAll(".mouse-per-line circle")
				.style("opacity", "0");
			d3.selectAll(".mouse-per-line text")
				.style("opacity", "0");
		})
		.on('mouseover', () => { // on mouse in show line, circles and text
			d3.select(".mouse-line")
				.style("opacity", "1");
			d3.selectAll(".mouse-per-line circle")
				.style("opacity", "1");
			d3.selectAll(".mouse-per-line text")
				.style("opacity", "1");
		})
		.on('mousemove', function () { // mouse moving over canvas
			const mouse = d3.mouse(this); // NOTE: `this` usage requires function(){}

			d3.select(".mouse-line").attr("d", () => `M${mouse[0]},${height} ${mouse[0]},0`);
			d3.selectAll(".mouse-per-line")
				.attr("transform", function (d, i) {
					var xDate = x.invert(mouse[0]),
						bisect = d3.bisector((d) => d.timestamp).right;
					let idx = bisect(d.values, xDate);

					let beginning = 0,
						end = lines[i].getTotalLength(),
						target = null;

					while (true) {
						target = Math.floor((beginning + end) / 2);
						pos = lines[i].getPointAtLength(target);
						if ((target === end || target === beginning) && pos.x !== mouse[0]) {
							break;
						}
						if (pos.x > mouse[0])      end = target;
						else if (pos.x < mouse[0]) beginning = target;
						else break; //position found
					}

					const label = d3.select(this).select('text');
					const playerCount = Math.floor(y.invert(pos.y));

					if (playerCount >= 1) {
						label.text(`${d.name}: ${playerCount}`);
					} else {
						label.text('');
					}

					return `translate(${mouse[0]}, ${pos.y})`;
				});
		});
})().catch(e => {
	console.log(e);
});
