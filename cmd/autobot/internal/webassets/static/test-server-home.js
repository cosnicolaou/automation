$(document).ready(function () {
    $("#reload-config").on("click", function (event) {
        event.preventDefault(); // Prevent the default link behavior
        $("#resultBar").empty();
        $.ajax({
            url: '/api/reload?',
            method: 'GET',
            success: function (data) {
                $('#resultBar').jsonViewer(data);
                window.location.reload(true);
            },
            error: function (jqXHR, textStatus, errorThrown) {
                console.error('operation failed:', textStatus, errorThrown);
                console.log(jqXHR);
                $('#resultBar').jsonViewer({ "Error": jqXHR.responseText, "Status": textStatus, "ErrorThrown": errorThrown });
            },
        });
    });
});
