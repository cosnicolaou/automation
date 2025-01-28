function runOperation(dev, op, params) {
    $("#resultBar").empty();
    let cgipars = params.map(function (value) {
        return { name: 'arg', value: value };
    });
    console.log('runOp', dev, op, cgipars);
    let cgiArgs = $.param([{ name: 'dev', value: dev }, { name: 'op', value: op }].concat(cgipars));
    $.ajax({
        url: '/api/operation?' + cgiArgs,
        method: 'GET',
        success: function (data) {
            $('#resultBar').jsonViewer(data, { collapsed: false });
        },
        error: function (jqXHR, textStatus, errorThrown) {
            console.error('operation failed:', textStatus, errorThrown);
            console.log(jqXHR);
            $('#resultBar').jsonViewer({ "Error": jqXHR.responseText, "Status": textStatus, "ErrorThrown": errorThrown });
        },
    });
}

function runCondition(dev, op, params) {
    $("#resultBar").empty();
    let cgipars = params.map(function (value) {
        return { name: 'arg', value: value };
    });
    console.log('runOp', dev, op, cgipars);
    let cgiArgs = $.param([{ name: 'dev', value: dev }, { name: 'op', value: op }].concat(cgipars));
    $.ajax({
        url: '/api/condition?' + cgiArgs,
        method: 'GET',
        success: function (data) {
            $('#resultBar').jsonViewer(data);
        },
        error: function (jqXHR, textStatus, errorThrown) {
            console.error('operation failed:', textStatus, errorThrown);
            console.log(jqXHR);
            $('#resultBar').jsonViewer({ "Error": jqXHR.responseText, "Status": textStatus, "ErrorThrown": errorThrown });
        },
    });
}

function loadStatusRecords(title, tag, endpoint, limit) {
    $.ajax({
        url: '/api/' + endpoint + '?' + $.param({ num: limit }),
        method: 'GET',
        success: function (data) {
            if (data.length > 0) {
                new gridjs.Grid({
                    title: title,
                    sort: true,
                    search: true,
                    columns: Object.keys(data[0]),
                    data: data,
                    pagination: {
                        limit: 100,
                    },
                    style: {
                        table: {
                            'white-space': 'nowrap'
                        }
                    },
                }).render(document.getElementById(tag));
            }
        }
    });
}

$('#reload-page').click(function (event) {
    event.preventDefault();
    location.reload();
    return false;
});


function loadCalendarRecords(from, to, schedules) {
    console.log('loadCalendarRecords', from, to, schedules);
    $.ajax({
        url: '/api/calendar?' + $.param({ from: from, to: to, schedules: schedules }),
        method: 'GET',
        success: function (data) {
            document.getElementById('daterange').innerHTML = `From: ${from} To: ${to}`;
            if (schedules.length > 0) {
                document.getElementById('schedules').innerHTML = `Schedules: ${schedules}`;
            } else {
                document.getElementById('schedules').innerHTML = `Schedules: All`;
            }
            if (data.calendar.length > 0) {
                new gridjs.Grid({
                    title: `From: ${from} To: ${to}`,
                    sort: true,
                    search: true,
                    columns: Object.keys(data.calendar[0]),
                    data: data.calendar,
                    pagination: {
                        limit: 100,
                    },
                    style: {
                        table: {
                            'white-space': 'nowrap'
                        }
                    },
                }).render(document.getElementById('calendar'));
            }
        },
        error: function (jqXHR, textStatus, errorThrown) {
            console.error('operation failed:', textStatus, errorThrown);
            console.log(jqXHR);
            $('#calendar').jsonViewer({ "Error": jqXHR.responseText, "Status": textStatus, "ErrorThrown": errorThrown });
        },
    });
}
